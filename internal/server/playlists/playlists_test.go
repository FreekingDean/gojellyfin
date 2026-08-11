package playlists

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/playlists"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/store"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
	playlistmodal "github.com/FreekingDean/gojellyfin/internal/store/playlist"
	entrymodal "github.com/FreekingDean/gojellyfin/internal/store/playlistentry"
	sharemodal "github.com/FreekingDean/gojellyfin/internal/store/playlistshare"
)

type fixture struct {
	server    *Server
	client    *store.Client
	libraryID uuid.UUID
	created   []uuid.UUID
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

	fixture := &fixture{
		server:    New(playlists.New(client), items.New(client)),
		client:    client,
		libraryID: library.ID,
	}

	t.Cleanup(func() {
		owned := playlistmodal.HasItemWith(itemmodal.IDIn(fixture.created...))
		if _, err := client.PlaylistShare.Delete().Where(sharemodal.HasPlaylistWith(owned)).Exec(ctx); err != nil {
			t.Errorf("failed to delete the shares: %v", err)
		}
		if _, err := client.PlaylistEntry.Delete().Where(entrymodal.HasPlaylistWith(owned)).Exec(ctx); err != nil {
			t.Errorf("failed to delete the entries: %v", err)
		}
		if _, err := client.Playlist.Delete().Where(owned).Exec(ctx); err != nil {
			t.Errorf("failed to delete the playlists: %v", err)
		}
		if _, err := client.Item.Delete().Where(itemmodal.IDIn(fixture.created...)).Exec(ctx); err != nil {
			t.Errorf("failed to delete the playlist items: %v", err)
		}
		if _, err := client.Item.Delete().Where(itemmodal.LibraryID(library.ID)).Exec(ctx); err != nil {
			t.Errorf("failed to delete the items: %v", err)
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

	response, err := f.server.CreatePlaylist(context.Background(), api.CreatePlaylistRequestObject{JSONBody: &body})
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

	response, err := f.server.GetPlaylistItems(context.Background(), api.GetPlaylistItemsRequestObject{PlaylistId: playlistID})
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
	ctx := context.Background()

	songs := fixture.songs(t, "One", "Two", "Three", "Four")
	playlistID := fixture.create(t, api.CreatePlaylistDto{
		Name:     ptr("Road Trip"),
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
		response, err := fixture.server.AddItemToPlaylist(ctx, api.AddItemToPlaylistRequestObject{
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
		response, err := fixture.server.MoveItem(ctx, api.MoveItemRequestObject{
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

		response, err := fixture.server.RemoveItemFromPlaylist(ctx, api.RemoveItemFromPlaylistRequestObject{
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
		response, err := fixture.server.GetPlaylist(ctx, api.GetPlaylistRequestObject{PlaylistId: playlistID})
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
		if len(*playlist.ItemIds) != 3 {
			t.Errorf("item ids = %d, want 3", len(*playlist.ItemIds))
		}
	})
}

func TestGetPlaylistNotFound(t *testing.T) {
	fixture := newFixture(t)

	response, err := fixture.server.GetPlaylist(context.Background(), api.GetPlaylistRequestObject{PlaylistId: uuid.New()})
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
