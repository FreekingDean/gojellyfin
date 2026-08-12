package playlists

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/store"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
	playlistmodal "github.com/FreekingDean/gojellyfin/internal/store/playlist"
	entrymodal "github.com/FreekingDean/gojellyfin/internal/store/playlistentry"
	sharemodal "github.com/FreekingDean/gojellyfin/internal/store/playlistshare"
	usermodal "github.com/FreekingDean/gojellyfin/internal/store/user"
)

type fixture struct {
	service   *Service
	client    *store.Client
	libraryID uuid.UUID
	ownerID   uuid.UUID
	guestID   uuid.UUID
	created   []uuid.UUID
	users     []uuid.UUID
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

	fixture := &fixture{service: New(client), client: client, libraryID: library.ID}
	fixture.ownerID = fixture.user(t, "owner")
	fixture.guestID = fixture.user(t, "guest")

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

func (f *fixture) user(t *testing.T, name string) uuid.UUID {
	t.Helper()

	record, err := f.client.User.Create().
		SetName(name).
		SetUsername(fmt.Sprintf("%s-%s-%s", t.Name(), name, uuid.NewString())).
		SetPasswordHash("hash").
		Save(context.Background())
	if err != nil {
		t.Fatalf("failed to create the %s user: %v", name, err)
	}
	f.users = append(f.users, record.ID)

	return record.ID
}

func (f *fixture) item(t *testing.T, name string, kind itemmodal.Kind, parentID *uuid.UUID, index *int32) uuid.UUID {
	t.Helper()

	record, err := f.client.Item.Create().
		SetLibraryID(f.libraryID).
		SetKind(kind).
		SetName(name).
		SetSortName(name).
		SetIsFolder(kind == itemmodal.KindSeries || kind == itemmodal.KindSeason).
		SetPath(fmt.Sprintf("/%s/%s", f.libraryID, name)).
		SetNillableParentID(parentID).
		SetNillableIndexNumber(index).
		Save(context.Background())
	if err != nil {
		t.Fatalf("failed to create %q: %v", name, err)
	}

	return record.ID
}

func (f *fixture) songs(t *testing.T, names ...string) []uuid.UUID {
	t.Helper()

	ids := make([]uuid.UUID, 0, len(names))
	for _, name := range names {
		ids = append(ids, f.item(t, name, itemmodal.KindAudio, nil, nil))
	}

	return ids
}

func (f *fixture) create(t *testing.T, params CreateParams) uuid.UUID {
	t.Helper()

	if params.MediaType == "" {
		params.MediaType = MediaTypeUnknown
	}
	if params.OwnerID == uuid.Nil {
		params.OwnerID = f.ownerID
	}

	item, err := f.service.Create(context.Background(), params)
	if err != nil {
		t.Fatalf("failed to create the playlist: %v", err)
	}
	f.created = append(f.created, item.ID)

	return item.ID
}

func (f *fixture) order(t *testing.T, playlistID uuid.UUID) []string {
	t.Helper()

	entries, _, err := f.service.Entries(context.Background(), playlistID, 0, 0)
	if err != nil {
		t.Fatalf("failed to query the entries: %v", err)
	}

	names := make([]string, 0, len(entries))
	for index, entry := range entries {
		if entry.SortOrder != int32(index) {
			t.Errorf("entry %d has sort order %d", index, entry.SortOrder)
		}
		names = append(names, entry.Edges.Item.Name)
	}

	return names
}

func (f *fixture) entryIDs(t *testing.T, playlistID uuid.UUID) []uuid.UUID {
	t.Helper()

	entries, _, err := f.service.Entries(context.Background(), playlistID, 0, 0)
	if err != nil {
		t.Fatalf("failed to query the entries: %v", err)
	}

	ids := make([]uuid.UUID, 0, len(entries))
	for _, entry := range entries {
		ids = append(ids, entry.ID)
	}

	return ids
}

func TestCreate(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()

	songs := fixture.songs(t, "One", "Two", "Three")
	playlistID := fixture.create(t, CreateParams{
		Name:       "Road Trip",
		MediaType:  "Audio",
		OpenAccess: true,
		ItemIDs:    songs,
		Shares:     []Permission{{UserID: fixture.guestID, CanEdit: true}},
	})

	playlist, err := fixture.service.PlaylistByItemID(ctx, playlistID)
	if err != nil {
		t.Fatalf("failed to query the playlist: %v", err)
	}
	if playlist.OwnerID != fixture.ownerID {
		t.Errorf("owner = %s, want %s", playlist.OwnerID, fixture.ownerID)
	}
	if !playlist.OpenAccess {
		t.Error("open access = false, want true")
	}

	want := []string{"One", "Two", "Three"}
	if got := fixture.order(t, playlistID); !slices.Equal(got, want) {
		t.Errorf("entries = %v, want %v", got, want)
	}

	shares, err := fixture.service.Shares(ctx, playlistID)
	if err != nil {
		t.Fatalf("failed to query the shares: %v", err)
	}
	if len(shares) != 1 || shares[0].UserID != fixture.guestID || !shares[0].CanEdit {
		t.Errorf("shares = %v, want one editable share for %s", shares, fixture.guestID)
	}
}

func TestAccess(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()

	playlistID := fixture.create(t, CreateParams{Name: "Private"})

	access := func(t *testing.T, userID uuid.UUID) Access {
		t.Helper()

		result, err := fixture.service.Access(ctx, playlistID, userID)
		if err != nil {
			t.Fatalf("failed to query access: %v", err)
		}

		return result
	}

	t.Run("the owner may do everything", func(t *testing.T) {
		if got := access(t, fixture.ownerID); got != (Access{Owner: true, CanEdit: true, CanView: true}) {
			t.Errorf("access = %+v, want full access", got)
		}
	})

	t.Run("a stranger may do nothing", func(t *testing.T) {
		if got := access(t, fixture.guestID); got != (Access{}) {
			t.Errorf("access = %+v, want no access", got)
		}
	})

	t.Run("a read-only share may view", func(t *testing.T) {
		if err := fixture.service.SetShare(ctx, playlistID, Permission{UserID: fixture.guestID}); err != nil {
			t.Fatalf("failed to add the share: %v", err)
		}

		if got := access(t, fixture.guestID); got != (Access{CanView: true}) {
			t.Errorf("access = %+v, want view only", got)
		}
	})

	t.Run("an editable share may edit but does not own", func(t *testing.T) {
		if err := fixture.service.SetShare(ctx, playlistID, Permission{UserID: fixture.guestID, CanEdit: true}); err != nil {
			t.Fatalf("failed to update the share: %v", err)
		}

		if got := access(t, fixture.guestID); got != (Access{CanEdit: true, CanView: true}) {
			t.Errorf("access = %+v, want edit without ownership", got)
		}
	})

	t.Run("an open playlist may be viewed by anyone", func(t *testing.T) {
		open := fixture.create(t, CreateParams{Name: "Open", OpenAccess: true})

		result, err := fixture.service.Access(ctx, open, fixture.guestID)
		if err != nil {
			t.Fatalf("failed to query access: %v", err)
		}
		if result != (Access{CanView: true}) {
			t.Errorf("access = %+v, want view only", result)
		}
	})
}

func TestAddItems(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()

	songs := fixture.songs(t, "One", "Two", "Three", "Four")
	playlistID := fixture.create(t, CreateParams{Name: "Mix", ItemIDs: songs[:2]})

	if err := fixture.service.AddItems(ctx, playlistID, songs[2:]); err != nil {
		t.Fatalf("failed to add the items: %v", err)
	}

	want := []string{"One", "Two", "Three", "Four"}
	if got := fixture.order(t, playlistID); !slices.Equal(got, want) {
		t.Errorf("entries = %v, want %v", got, want)
	}
}

func TestFoldersExpandInIndexOrder(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()

	series := fixture.item(t, "Series", itemmodal.KindSeries, nil, nil)
	first := fixture.item(t, "Bravo", itemmodal.KindSeason, &series, ptr(int32(1)))
	second := fixture.item(t, "Alpha", itemmodal.KindSeason, &series, ptr(int32(2)))
	fixture.item(t, "Second", itemmodal.KindEpisode, &first, ptr(int32(1)))
	fixture.item(t, "First", itemmodal.KindEpisode, &first, ptr(int32(2)))
	fixture.item(t, "Solo", itemmodal.KindEpisode, &second, ptr(int32(1)))

	song := fixture.songs(t, "Zulu")
	ids := []uuid.UUID{series, song[0]}
	want := []string{"Second", "First", "Solo", "Zulu"}

	t.Run("adding a folder adds what is beneath it", func(t *testing.T) {
		playlistID := fixture.create(t, CreateParams{Name: "Binge"})

		if err := fixture.service.AddItems(ctx, playlistID, ids); err != nil {
			t.Fatalf("failed to add the items: %v", err)
		}
		if got := fixture.order(t, playlistID); !slices.Equal(got, want) {
			t.Errorf("entries = %v, want %v", got, want)
		}
	})

	t.Run("replacing the items expands them too", func(t *testing.T) {
		playlistID := fixture.create(t, CreateParams{Name: "Rebinge"})

		if err := fixture.service.Update(ctx, playlistID, UpdateParams{ItemIDs: &ids}); err != nil {
			t.Fatalf("failed to update the playlist: %v", err)
		}
		if got := fixture.order(t, playlistID); !slices.Equal(got, want) {
			t.Errorf("entries = %v, want %v", got, want)
		}
	})
}

func TestUnknownItemsAreIgnored(t *testing.T) {
	fixture := newFixture(t)

	songs := fixture.songs(t, "One")
	playlistID := fixture.create(t, CreateParams{
		Name:    "Ghosts",
		ItemIDs: []uuid.UUID{uuid.New(), songs[0], uuid.New()},
	})

	want := []string{"One"}
	if got := fixture.order(t, playlistID); !slices.Equal(got, want) {
		t.Errorf("entries = %v, want %v", got, want)
	}
}

func TestAddItemsKeepsDuplicates(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()

	songs := fixture.songs(t, "One")
	playlistID := fixture.create(t, CreateParams{Name: "Repeat", ItemIDs: songs})

	if err := fixture.service.AddItems(ctx, playlistID, songs); err != nil {
		t.Fatalf("failed to add the items: %v", err)
	}

	want := []string{"One", "One"}
	if got := fixture.order(t, playlistID); !slices.Equal(got, want) {
		t.Errorf("entries = %v, want %v", got, want)
	}

	ids := fixture.entryIDs(t, playlistID)
	if ids[0] == ids[1] {
		t.Error("the two entries share an id")
	}
}

func TestMoveEntry(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()

	songs := fixture.songs(t, "One", "Two", "Three", "Four")
	playlistID := fixture.create(t, CreateParams{Name: "Shuffle", ItemIDs: songs})
	ids := fixture.entryIDs(t, playlistID)

	t.Run("moves an entry to the front", func(t *testing.T) {
		if err := fixture.service.MoveEntry(ctx, playlistID, ids[3], 0); err != nil {
			t.Fatalf("failed to move the entry: %v", err)
		}

		want := []string{"Four", "One", "Two", "Three"}
		if got := fixture.order(t, playlistID); !slices.Equal(got, want) {
			t.Errorf("entries = %v, want %v", got, want)
		}
	})

	t.Run("moves an entry into the middle", func(t *testing.T) {
		if err := fixture.service.MoveEntry(ctx, playlistID, ids[0], 2); err != nil {
			t.Fatalf("failed to move the entry: %v", err)
		}

		want := []string{"Four", "Two", "One", "Three"}
		if got := fixture.order(t, playlistID); !slices.Equal(got, want) {
			t.Errorf("entries = %v, want %v", got, want)
		}
	})

	t.Run("rejects an index outside the playlist", func(t *testing.T) {
		for _, index := range []int{-1, 4, 99} {
			if err := fixture.service.MoveEntry(ctx, playlistID, ids[1], index); err == nil {
				t.Errorf("moving to index %d succeeded", index)
			}
		}

		want := []string{"Four", "Two", "One", "Three"}
		if got := fixture.order(t, playlistID); !slices.Equal(got, want) {
			t.Errorf("entries = %v, want %v", got, want)
		}
	})

	t.Run("moves an entry to the last index", func(t *testing.T) {
		if err := fixture.service.MoveEntry(ctx, playlistID, ids[1], 3); err != nil {
			t.Fatalf("failed to move the entry: %v", err)
		}

		want := []string{"Four", "One", "Three", "Two"}
		if got := fixture.order(t, playlistID); !slices.Equal(got, want) {
			t.Errorf("entries = %v, want %v", got, want)
		}
	})

	t.Run("rejects an entry from another playlist", func(t *testing.T) {
		if err := fixture.service.MoveEntry(ctx, playlistID, uuid.New(), 0); err == nil {
			t.Error("moving an unknown entry succeeded")
		}
	})
}

func TestRemoveEntries(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()

	songs := fixture.songs(t, "One", "Two", "Three", "Four")
	playlistID := fixture.create(t, CreateParams{Name: "Trim", ItemIDs: songs})
	ids := fixture.entryIDs(t, playlistID)

	if err := fixture.service.RemoveEntries(ctx, playlistID, []uuid.UUID{ids[0], ids[2]}); err != nil {
		t.Fatalf("failed to remove the entries: %v", err)
	}

	want := []string{"Two", "Four"}
	if got := fixture.order(t, playlistID); !slices.Equal(got, want) {
		t.Errorf("entries = %v, want %v", got, want)
	}

	if err := fixture.service.RemoveEntries(ctx, playlistID, nil); err != nil {
		t.Fatalf("failed to remove no entries: %v", err)
	}
	if got := fixture.order(t, playlistID); !slices.Equal(got, want) {
		t.Errorf("entries = %v after removing nothing, want %v", got, want)
	}
}

func TestEntriesPages(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()

	songs := fixture.songs(t, "One", "Two", "Three", "Four")
	playlistID := fixture.create(t, CreateParams{Name: "Paged", ItemIDs: songs})

	entries, total, err := fixture.service.Entries(ctx, playlistID, 1, 2)
	if err != nil {
		t.Fatalf("failed to query the entries: %v", err)
	}
	if total != 4 {
		t.Errorf("total = %d, want 4", total)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Edges.Item.Name)
	}
	if want := []string{"Two", "Three"}; !slices.Equal(names, want) {
		t.Errorf("entries = %v, want %v", names, want)
	}
}

func TestUpdate(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()

	songs := fixture.songs(t, "One", "Two", "Three")
	playlistID := fixture.create(t, CreateParams{Name: "Before", ItemIDs: songs[:1]})

	replacement := []uuid.UUID{songs[2], songs[1]}
	err := fixture.service.Update(ctx, playlistID, UpdateParams{
		Name:       ptr("After"),
		OpenAccess: ptr(true),
		ItemIDs:    &replacement,
		Shares:     &[]Permission{{UserID: fixture.guestID}},
	})
	if err != nil {
		t.Fatalf("failed to update the playlist: %v", err)
	}

	item, err := fixture.client.Item.Get(ctx, playlistID)
	if err != nil {
		t.Fatalf("failed to query the playlist item: %v", err)
	}
	if item.Name != "After" {
		t.Errorf("name = %q, want %q", item.Name, "After")
	}

	want := []string{"Three", "Two"}
	if got := fixture.order(t, playlistID); !slices.Equal(got, want) {
		t.Errorf("entries = %v, want %v", got, want)
	}

	shares, err := fixture.service.Shares(ctx, playlistID)
	if err != nil {
		t.Fatalf("failed to query the shares: %v", err)
	}
	if len(shares) != 1 || shares[0].CanEdit {
		t.Errorf("shares = %v, want one read-only share", shares)
	}
}

func TestShares(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()

	playlistID := fixture.create(t, CreateParams{Name: "Shared"})

	if err := fixture.service.SetShare(ctx, playlistID, Permission{UserID: fixture.guestID}); err != nil {
		t.Fatalf("failed to add the share: %v", err)
	}
	if err := fixture.service.SetShare(ctx, playlistID, Permission{UserID: fixture.guestID, CanEdit: true}); err != nil {
		t.Fatalf("failed to update the share: %v", err)
	}

	share, err := fixture.service.ShareFor(ctx, playlistID, fixture.guestID)
	if err != nil {
		t.Fatalf("failed to query the share: %v", err)
	}
	if !share.CanEdit {
		t.Error("can edit = false, want true")
	}

	shares, err := fixture.service.Shares(ctx, playlistID)
	if err != nil {
		t.Fatalf("failed to query the shares: %v", err)
	}
	if len(shares) != 1 {
		t.Errorf("shares = %d, want 1", len(shares))
	}

	if err := fixture.service.RemoveShare(ctx, playlistID, fixture.guestID); err != nil {
		t.Fatalf("failed to remove the share: %v", err)
	}
	if _, err := fixture.service.ShareFor(ctx, playlistID, fixture.guestID); err == nil {
		t.Error("querying the removed share succeeded")
	}
}

func TestSharesAreValidated(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()

	playlistID := fixture.create(t, CreateParams{
		Name:   "Guarded",
		Shares: []Permission{{UserID: fixture.guestID}},
	})

	rejected := map[string][]Permission{
		"no user id":   {{UserID: uuid.Nil}},
		"unknown user": {{UserID: uuid.New()}},
		"the same user twice": {
			{UserID: fixture.guestID},
			{UserID: fixture.guestID, CanEdit: true},
		},
	}

	for name, permissions := range rejected {
		t.Run(name, func(t *testing.T) {
			err := fixture.service.Update(ctx, playlistID, UpdateParams{Shares: &permissions})
			if !errors.Is(err, ErrInvalidShare) {
				t.Fatalf("error = %v, want %v", err, ErrInvalidShare)
			}

			shares, err := fixture.service.Shares(ctx, playlistID)
			if err != nil {
				t.Fatalf("failed to query the shares: %v", err)
			}
			if len(shares) != 1 || shares[0].UserID != fixture.guestID || shares[0].CanEdit {
				t.Errorf("shares = %v, want the original read-only share intact", shares)
			}
		})
	}

	t.Run("a single unknown share", func(t *testing.T) {
		err := fixture.service.SetShare(ctx, playlistID, Permission{UserID: uuid.New()})
		if !errors.Is(err, ErrInvalidShare) {
			t.Fatalf("error = %v, want %v", err, ErrInvalidShare)
		}
	})
}

func TestCreateRejectsAnUnknownShare(t *testing.T) {
	fixture := newFixture(t)

	_, err := fixture.service.Create(context.Background(), CreateParams{
		Name:      "Doomed",
		MediaType: MediaTypeUnknown,
		OwnerID:   fixture.ownerID,
		Shares:    []Permission{{UserID: uuid.New()}},
	})
	if !errors.Is(err, ErrInvalidShare) {
		t.Fatalf("error = %v, want %v", err, ErrInvalidShare)
	}
}

func ptr[T any](value T) *T {
	return &value
}
