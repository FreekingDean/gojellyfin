package playlists

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/activity"
	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/playlists"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/sessions"
	"github.com/FreekingDean/gojellyfin/internal/store"
	devicemodal "github.com/FreekingDean/gojellyfin/internal/store/device"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
	playlistmodal "github.com/FreekingDean/gojellyfin/internal/store/playlist"
	entrymodal "github.com/FreekingDean/gojellyfin/internal/store/playlistentry"
	sharemodal "github.com/FreekingDean/gojellyfin/internal/store/playlistshare"
	sessionmodal "github.com/FreekingDean/gojellyfin/internal/store/session"
	usermodal "github.com/FreekingDean/gojellyfin/internal/store/user"
)

type fixture struct {
	server    *Server
	client    *store.Client
	auth      *auth.Service
	sessions  *sessions.Service
	libraryID uuid.UUID
	owner     context.Context
	guest     context.Context
	guestID   uuid.UUID
	created   []uuid.UUID
	users     []uuid.UUID
	devices   []string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	connection, err := store.NewStore()
	if err != nil {
		t.Fatalf("failed to open the database: %v", err)
	}
	if err := connection.Start(); err != nil {
		t.Fatalf("failed to reach the database, set DATABASE_URL: %v", err)
	}

	ctx := context.Background()
	client := connection.Client()

	library, err := client.Library.Create().SetName(t.Name() + "-" + uuid.NewString()).Save(ctx)
	if err != nil {
		t.Fatalf("failed to create the library: %v", err)
	}

	sessionService := sessions.New(client, activity.New(client))
	fixture := &fixture{
		server:    New(playlists.New(client), items.New(client)),
		client:    client,
		auth:      auth.New(sessionService),
		sessions:  sessionService,
		libraryID: library.ID,
	}

	_, fixture.owner = fixture.signIn(t, "owner")
	fixture.guestID, fixture.guest = fixture.signIn(t, "guest")

	t.Cleanup(func() {
		if _, err := client.PlaylistShare.Delete().
			Where(sharemodal.HasPlaylistWith(playlistmodal.ItemIDIn(fixture.created...))).
			Exec(ctx); err != nil {
			t.Errorf("failed to delete the shares: %v", err)
		}
		if _, err := client.PlaylistEntry.Delete().
			Where(entrymodal.HasPlaylistWith(playlistmodal.ItemIDIn(fixture.created...))).
			Exec(ctx); err != nil {
			t.Errorf("failed to delete the entries: %v", err)
		}
		if _, err := client.Playlist.Delete().Where(playlistmodal.ItemIDIn(fixture.created...)).Exec(ctx); err != nil {
			t.Errorf("failed to delete the playlists: %v", err)
		}
		if _, err := client.Item.Delete().Where(itemmodal.IDIn(fixture.created...)).Exec(ctx); err != nil {
			t.Errorf("failed to delete the playlist items: %v", err)
		}
		if _, err := client.Item.Delete().Where(itemmodal.LibraryID(library.ID)).Exec(ctx); err != nil {
			t.Errorf("failed to delete the items: %v", err)
		}
		if _, err := client.Session.Delete().
			Where(sessionmodal.HasUserWith(usermodal.IDIn(fixture.users...))).
			Exec(ctx); err != nil {
			t.Errorf("failed to delete the sessions: %v", err)
		}
		if _, err := client.Device.Delete().Where(devicemodal.ClientIDIn(fixture.devices...)).Exec(ctx); err != nil {
			t.Errorf("failed to delete the devices: %v", err)
		}
		if _, err := client.User.Delete().Where(usermodal.IDIn(fixture.users...)).Exec(ctx); err != nil {
			t.Errorf("failed to delete the users: %v", err)
		}
		if err := client.Library.DeleteOne(library).Exec(ctx); err != nil {
			t.Errorf("failed to delete the library: %v", err)
		}
		if err := connection.Stop(); err != nil {
			t.Errorf("failed to close the database: %v", err)
		}
	})

	return fixture
}

func (f *fixture) signIn(t *testing.T, name string) (uuid.UUID, context.Context) {
	t.Helper()

	ctx := context.Background()
	unique := uuid.NewString()

	user, err := f.client.User.Create().
		SetName(name).
		SetUsername(fmt.Sprintf("%s-%s-%s", t.Name(), name, unique)).
		SetPasswordHash("hash").
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to create the %s user: %v", name, err)
	}
	f.users = append(f.users, user.ID)

	device := sessions.DeviceInfo{ID: unique, Name: name, AppName: "test", AppVersion: "1"}
	f.devices = append(f.devices, unique)

	if _, err := f.sessions.Create(ctx, user.ID, unique, device); err != nil {
		t.Fatalf("failed to create the %s session: %v", name, err)
	}

	authenticated, err := f.auth.Authenticate(ctx, unique)
	if err != nil {
		t.Fatalf("failed to authenticate the %s session: %v", name, err)
	}

	return user.ID, authenticated
}

func (f *fixture) songs(t *testing.T, names ...string) []uuid.UUID {
	t.Helper()

	ids := make([]uuid.UUID, 0, len(names))
	for _, name := range names {
		record, err := f.client.Item.Create().
			SetLibraryID(f.libraryID).
			SetKind(itemmodal.KindAudio).
			SetName(name).
			SetSortName(name).
			SetPath(fmt.Sprintf("/%s/%s", f.libraryID, name)).
			Save(context.Background())
		if err != nil {
			t.Fatalf("failed to create %q: %v", name, err)
		}
		ids = append(ids, record.ID)
	}

	return ids
}

func (f *fixture) create(t *testing.T, body api.CreatePlaylistDto) uuid.UUID {
	t.Helper()

	response, err := f.server.CreatePlaylist(f.owner, api.CreatePlaylistRequestObject{JSONBody: &body})
	if err != nil {
		t.Fatalf("failed to create the playlist: %v", err)
	}

	created, ok := response.(api.CreatePlaylist200JSONResponse)
	if !ok {
		t.Fatalf("CreatePlaylist returned %T", response)
	}

	id, err := uuid.Parse(*created.Id)
	if err != nil {
		t.Fatalf("failed to parse the playlist id: %v", err)
	}
	f.created = append(f.created, id)

	return id
}

func (f *fixture) items(t *testing.T, playlistID uuid.UUID) []api.BaseItemDto {
	t.Helper()

	response, err := f.server.GetPlaylistItems(f.owner, api.GetPlaylistItemsRequestObject{PlaylistId: playlistID})
	if err != nil {
		t.Fatalf("failed to query the playlist items: %v", err)
	}

	result, ok := response.(api.GetPlaylistItems200JSONResponse)
	if !ok {
		t.Fatalf("GetPlaylistItems returned %T", response)
	}

	return *result.Items
}

func names(records []api.BaseItemDto) []string {
	found := make([]string, 0, len(records))
	for _, record := range records {
		found = append(found, *record.Name)
	}

	return found
}

func TestPlaylistRoundTrip(t *testing.T) {
	fixture := newFixture(t)

	songs := fixture.songs(t, "One", "Two", "Three", "Four")
	playlistID := fixture.create(t, api.CreatePlaylistDto{
		Name:     "Road Trip",
		Ids:      &songs,
		IsPublic: ptr(true),
	})

	t.Run("returns the items in order with their entry ids", func(t *testing.T) {
		records := fixture.items(t, playlistID)

		want := []string{"One", "Two", "Three", "Four"}
		if got := names(records); !slices.Equal(got, want) {
			t.Errorf("items = %v, want %v", got, want)
		}
		for _, record := range records {
			if record.PlaylistItemId == nil {
				t.Fatalf("%q has no playlist item id", *record.Name)
			}
			if _, err := uuid.Parse(*record.PlaylistItemId); err != nil {
				t.Errorf("playlist item id %q is not a uuid", *record.PlaylistItemId)
			}
		}
	})

	t.Run("adds items to the end", func(t *testing.T) {
		extra := fixture.songs(t, "Five")
		response, err := fixture.server.AddItemToPlaylist(fixture.owner, api.AddItemToPlaylistRequestObject{
			PlaylistId: playlistID,
			Params:     api.AddItemToPlaylistParams{Ids: &extra},
		})
		if err != nil {
			t.Fatalf("failed to add the item: %v", err)
		}
		if _, ok := response.(api.AddItemToPlaylist204Response); !ok {
			t.Fatalf("AddItemToPlaylist returned %T", response)
		}

		want := []string{"One", "Two", "Three", "Four", "Five"}
		if got := names(fixture.items(t, playlistID)); !slices.Equal(got, want) {
			t.Errorf("items = %v, want %v", got, want)
		}
	})

	t.Run("moves an entry by its entry id", func(t *testing.T) {
		records := fixture.items(t, playlistID)
		response, err := fixture.server.MoveItem(fixture.owner, api.MoveItemRequestObject{
			PlaylistId: playlistID.String(),
			ItemId:     *records[4].PlaylistItemId,
			NewIndex:   1,
		})
		if err != nil {
			t.Fatalf("failed to move the entry: %v", err)
		}
		if _, ok := response.(api.MoveItem204Response); !ok {
			t.Fatalf("MoveItem returned %T", response)
		}

		want := []string{"One", "Five", "Two", "Three", "Four"}
		if got := names(fixture.items(t, playlistID)); !slices.Equal(got, want) {
			t.Errorf("items = %v, want %v", got, want)
		}
	})

	t.Run("removes entries by their entry ids", func(t *testing.T) {
		records := fixture.items(t, playlistID)
		entryIDs := []string{*records[0].PlaylistItemId, *records[2].PlaylistItemId}

		response, err := fixture.server.RemoveItemFromPlaylist(fixture.owner, api.RemoveItemFromPlaylistRequestObject{
			PlaylistId: playlistID.String(),
			Params:     api.RemoveItemFromPlaylistParams{EntryIds: &entryIDs},
		})
		if err != nil {
			t.Fatalf("failed to remove the entries: %v", err)
		}
		if _, ok := response.(api.RemoveItemFromPlaylist204Response); !ok {
			t.Fatalf("RemoveItemFromPlaylist returned %T", response)
		}

		want := []string{"Five", "Three", "Four"}
		if got := names(fixture.items(t, playlistID)); !slices.Equal(got, want) {
			t.Errorf("items = %v, want %v", got, want)
		}
	})

	t.Run("reports the remaining items and access", func(t *testing.T) {
		response, err := fixture.server.GetPlaylist(fixture.owner, api.GetPlaylistRequestObject{PlaylistId: playlistID})
		if err != nil {
			t.Fatalf("failed to query the playlist: %v", err)
		}

		playlist, ok := response.(api.GetPlaylist200JSONResponse)
		if !ok {
			t.Fatalf("GetPlaylist returned %T", response)
		}
		if !*playlist.OpenAccess {
			t.Error("open access = false, want true")
		}
		if len(playlist.ItemIds) != 3 {
			t.Errorf("item ids = %d, want 3", len(playlist.ItemIds))
		}
	})
}

func TestPermissions(t *testing.T) {
	fixture := newFixture(t)

	songs := fixture.songs(t, "One")
	playlistID := fixture.create(t, api.CreatePlaylistDto{Name: "Private", Ids: &songs})

	share := func(t *testing.T, canEdit bool) {
		t.Helper()

		response, err := fixture.server.UpdatePlaylistUser(fixture.owner, api.UpdatePlaylistUserRequestObject{
			PlaylistId: playlistID,
			UserId:     fixture.guestID,
			JSONBody:   &api.UpdatePlaylistUserDto{CanEdit: ptr(canEdit)},
		})
		if err != nil {
			t.Fatalf("failed to share the playlist: %v", err)
		}
		if _, ok := response.(api.UpdatePlaylistUser204Response); !ok {
			t.Fatalf("UpdatePlaylistUser returned %T", response)
		}
	}

	add := func(t *testing.T, ctx context.Context) api.AddItemToPlaylistResponseObject {
		t.Helper()

		extra := fixture.songs(t, uuid.NewString())
		response, err := fixture.server.AddItemToPlaylist(ctx, api.AddItemToPlaylistRequestObject{
			PlaylistId: playlistID,
			Params:     api.AddItemToPlaylistParams{Ids: &extra},
		})
		if err != nil {
			t.Fatalf("failed to add the item: %v", err)
		}

		return response
	}

	t.Run("an unauthenticated caller may not create", func(t *testing.T) {
		response, err := fixture.server.CreatePlaylist(context.Background(), api.CreatePlaylistRequestObject{
			JSONBody: &api.CreatePlaylistDto{Name: "Anonymous"},
		})
		if err != nil {
			t.Fatalf("failed to create the playlist: %v", err)
		}
		if _, ok := response.(api.CreatePlaylist403Response); !ok {
			t.Fatalf("CreatePlaylist returned %T, want a 403", response)
		}
	})

	t.Run("a stranger may not read or write", func(t *testing.T) {
		response, err := fixture.server.GetPlaylist(fixture.guest, api.GetPlaylistRequestObject{PlaylistId: playlistID})
		if err != nil {
			t.Fatalf("failed to query the playlist: %v", err)
		}
		if _, ok := response.(api.GetPlaylist403Response); !ok {
			t.Fatalf("GetPlaylist returned %T, want a 403", response)
		}

		if got := add(t, fixture.guest); got == nil {
			t.Fatal("AddItemToPlaylist returned nothing")
		} else if _, ok := got.(api.AddItemToPlaylist403JSONResponse); !ok {
			t.Fatalf("AddItemToPlaylist returned %T, want a 403", got)
		}
	})

	t.Run("a read-only share may read but not write", func(t *testing.T) {
		share(t, false)

		response, err := fixture.server.GetPlaylistItems(fixture.guest, api.GetPlaylistItemsRequestObject{PlaylistId: playlistID})
		if err != nil {
			t.Fatalf("failed to query the playlist items: %v", err)
		}
		if _, ok := response.(api.GetPlaylistItems200JSONResponse); !ok {
			t.Fatalf("GetPlaylistItems returned %T, want a 200", response)
		}

		if got := add(t, fixture.guest); got == nil {
			t.Fatal("AddItemToPlaylist returned nothing")
		} else if _, ok := got.(api.AddItemToPlaylist403JSONResponse); !ok {
			t.Fatalf("AddItemToPlaylist returned %T, want a 403", got)
		}
	})

	t.Run("an editable share may write", func(t *testing.T) {
		share(t, true)

		if _, ok := add(t, fixture.guest).(api.AddItemToPlaylist204Response); !ok {
			t.Error("an editable share could not add an item")
		}
	})

	t.Run("only the owner manages users", func(t *testing.T) {
		response, err := fixture.server.GetPlaylistUsers(fixture.guest, api.GetPlaylistUsersRequestObject{PlaylistId: playlistID})
		if err != nil {
			t.Fatalf("failed to query the playlist users: %v", err)
		}
		if _, ok := response.(api.GetPlaylistUsers403JSONResponse); !ok {
			t.Fatalf("GetPlaylistUsers returned %T, want a 403", response)
		}

		removal, err := fixture.server.RemoveUserFromPlaylist(fixture.guest, api.RemoveUserFromPlaylistRequestObject{
			PlaylistId: playlistID,
			UserId:     fixture.guestID,
		})
		if err != nil {
			t.Fatalf("failed to remove the user: %v", err)
		}
		if _, ok := removal.(api.RemoveUserFromPlaylist403JSONResponse); !ok {
			t.Fatalf("RemoveUserFromPlaylist returned %T, want a 403", removal)
		}

		owned, err := fixture.server.GetPlaylistUsers(fixture.owner, api.GetPlaylistUsersRequestObject{PlaylistId: playlistID})
		if err != nil {
			t.Fatalf("failed to query the playlist users: %v", err)
		}
		users, ok := owned.(api.GetPlaylistUsers200JSONResponse)
		if !ok {
			t.Fatalf("GetPlaylistUsers returned %T, want a 200", owned)
		}
		if len(users) != 1 || *users[0].UserId != fixture.guestID || !*users[0].CanEdit {
			t.Errorf("users = %v, want one editable share for %s", users, fixture.guestID)
		}
	})
}

func TestBogusSharesAreRefused(t *testing.T) {
	fixture := newFixture(t)

	playlistID := fixture.create(t, api.CreatePlaylistDto{
		Name:  "Guarded",
		Users: &[]api.PlaylistUserPermissions{{UserId: &fixture.guestID}},
	})

	t.Run("create refuses an unknown user", func(t *testing.T) {
		unknown := uuid.New()
		response, err := fixture.server.CreatePlaylist(fixture.owner, api.CreatePlaylistRequestObject{
			JSONBody: &api.CreatePlaylistDto{
				Name:  "Doomed",
				Users: &[]api.PlaylistUserPermissions{{UserId: &unknown}},
			},
		})
		if err != nil {
			t.Fatalf("failed to create the playlist: %v", err)
		}
		if _, ok := response.(api.CreatePlaylist403Response); !ok {
			t.Fatalf("CreatePlaylist returned %T, want a 403", response)
		}
	})

	t.Run("update refuses a share without a user id", func(t *testing.T) {
		response, err := fixture.server.UpdatePlaylist(fixture.owner, api.UpdatePlaylistRequestObject{
			PlaylistId: playlistID,
			JSONBody:   &api.UpdatePlaylistDto{Users: &[]api.PlaylistUserPermissions{{CanEdit: ptr(true)}}},
		})
		if err != nil {
			t.Fatalf("failed to update the playlist: %v", err)
		}
		if _, ok := response.(api.UpdatePlaylist403JSONResponse); !ok {
			t.Fatalf("UpdatePlaylist returned %T, want a 403", response)
		}
	})

	t.Run("the original shares survive", func(t *testing.T) {
		response, err := fixture.server.GetPlaylistUsers(fixture.owner, api.GetPlaylistUsersRequestObject{PlaylistId: playlistID})
		if err != nil {
			t.Fatalf("failed to query the playlist users: %v", err)
		}

		users, ok := response.(api.GetPlaylistUsers200JSONResponse)
		if !ok {
			t.Fatalf("GetPlaylistUsers returned %T", response)
		}
		if len(users) != 1 || *users[0].UserId != fixture.guestID {
			t.Errorf("users = %v, want the original share for %s", users, fixture.guestID)
		}
	})

	t.Run("sharing with an unknown user is a 404", func(t *testing.T) {
		response, err := fixture.server.UpdatePlaylistUser(fixture.owner, api.UpdatePlaylistUserRequestObject{
			PlaylistId: playlistID,
			UserId:     uuid.New(),
			JSONBody:   &api.UpdatePlaylistUserDto{CanEdit: ptr(true)},
		})
		if err != nil {
			t.Fatalf("failed to share the playlist: %v", err)
		}
		if _, ok := response.(api.UpdatePlaylistUser404JSONResponse); !ok {
			t.Fatalf("UpdatePlaylistUser returned %T, want a 404", response)
		}
	})
}

func TestGetPlaylistNotFound(t *testing.T) {
	fixture := newFixture(t)

	response, err := fixture.server.GetPlaylist(fixture.owner, api.GetPlaylistRequestObject{PlaylistId: uuid.New()})
	if err != nil {
		t.Fatalf("failed to query the playlist: %v", err)
	}
	if _, ok := response.(api.GetPlaylist404JSONResponse); !ok {
		t.Fatalf("GetPlaylist returned %T, want a 404", response)
	}
}

func ptr[T any](value T) *T {
	return &value
}
