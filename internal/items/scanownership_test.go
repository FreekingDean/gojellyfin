package items

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
)

func (f *fixture) scan(t *testing.T, name string, year *int32) *Item {
	t.Helper()

	record, err := f.service.SaveScanned(context.Background(), Scanned{
		LibraryID:      f.libraryID,
		Kind:           itemmodal.KindMovie,
		Key:            "movie:rescanned",
		Name:           name,
		SortName:       name,
		ProductionYear: year,
		DateModified:   time.Now(),
	})
	if err != nil {
		t.Fatalf("failed to scan %q: %v", name, err)
	}

	return record
}

func TestService_SaveScannedTitleOwnership(t *testing.T) {
	t.Run("renames an item nothing else has claimed", func(t *testing.T) {
		fixture := newFixture(t)

		first := fixture.scan(t, "the matrix", number(1999))
		second := fixture.scan(t, "The Matrix", number(2000))

		if second.ID != first.ID {
			t.Fatalf("the rescan created a second item: %s, was %s", second.ID, first.ID)
		}
		if second.Name != "The Matrix" {
			t.Errorf("name = %q, want the renamed file's title", second.Name)
		}
		if second.SortName != "The Matrix" {
			t.Errorf("sort name = %q, want the renamed file's title", second.SortName)
		}
		if second.ProductionYear == nil || *second.ProductionYear != 2000 {
			t.Errorf("production year = %v, want 2000", year(second.ProductionYear))
		}
	})

	t.Run("keeps what identification wrote", func(t *testing.T) {
		fixture := newFixture(t)
		ctx := context.Background()

		scanned := fixture.scan(t, "Series S01E01", number(1999))

		if _, err := fixture.service.UpdateMetadata(ctx, scanned.ID, Metadata{
			Name:           ptr("Pilot"),
			SortName:       ptr("pilot"),
			ProductionYear: number(2002),
			ProviderIds:    &map[string]string{"Tmdb": "1438"},
		}); err != nil {
			t.Fatalf("failed to identify the item: %v", err)
		}

		rescanned := fixture.scan(t, "Series S01E01", number(1999))

		if rescanned.Name != "Pilot" {
			t.Errorf("name = %q, want the identified title to survive a rescan", rescanned.Name)
		}
		if rescanned.SortName != "pilot" {
			t.Errorf("sort name = %q, want the identified sort name to survive a rescan", rescanned.SortName)
		}
		if rescanned.ProductionYear == nil || *rescanned.ProductionYear != 2002 {
			t.Errorf("production year = %v, want the identified year to survive a rescan", year(rescanned.ProductionYear))
		}
	})

	t.Run("keeps what an editor claimed", func(t *testing.T) {
		fixture := newFixture(t)
		ctx := context.Background()

		scanned := fixture.scan(t, "the matrix", number(1999))

		if _, err := fixture.service.EditMetadata(ctx, scanned, Metadata{
			Name:           ptr("The Matrix"),
			ProductionYear: number(2000),
		}); err != nil {
			t.Fatalf("failed to edit the item: %v", err)
		}

		rescanned := fixture.scan(t, "the matrix", number(1999))

		if rescanned.Name != "The Matrix" {
			t.Errorf("name = %q, want the edited title to survive a rescan", rescanned.Name)
		}
		if rescanned.ProductionYear == nil || *rescanned.ProductionYear != 2000 {
			t.Errorf("production year = %v, want the edited year to survive a rescan", year(rescanned.ProductionYear))
		}
	})

	t.Run("keeps a locked title", func(t *testing.T) {
		fixture := newFixture(t)
		ctx := context.Background()

		scanned := fixture.scan(t, "the matrix", nil)

		if _, err := fixture.service.UpdateMetadata(ctx, scanned.ID, Metadata{
			Name:         ptr("The Matrix"),
			LockedFields: &[]string{LockedName},
		}); err != nil {
			t.Fatalf("failed to lock the item: %v", err)
		}

		if rescanned := fixture.scan(t, "the matrix", nil); rescanned.Name != "The Matrix" {
			t.Errorf("name = %q, want a locked title to survive a rescan", rescanned.Name)
		}
	})

	t.Run("keeps the title of an item locked from changes", func(t *testing.T) {
		fixture := newFixture(t)
		ctx := context.Background()

		scanned := fixture.scan(t, "the matrix", nil)

		if _, err := fixture.service.UpdateMetadata(ctx, scanned.ID, Metadata{
			Name:     ptr("The Matrix"),
			LockData: ptr(true),
		}); err != nil {
			t.Fatalf("failed to lock the item: %v", err)
		}

		if rescanned := fixture.scan(t, "the matrix", nil); rescanned.Name != "The Matrix" {
			t.Errorf("name = %q, want a locked item's title to survive a rescan", rescanned.Name)
		}
	})

	t.Run("renames an item locked on another field", func(t *testing.T) {
		fixture := newFixture(t)
		ctx := context.Background()

		scanned := fixture.scan(t, "the matrix", nil)

		if _, err := fixture.service.UpdateMetadata(ctx, scanned.ID, Metadata{
			LockedFields: &[]string{"Overview"},
		}); err != nil {
			t.Fatalf("failed to lock the item: %v", err)
		}

		if rescanned := fixture.scan(t, "The Matrix", nil); rescanned.Name != "The Matrix" {
			t.Errorf("name = %q, want an overview lock to leave the title scanner owned", rescanned.Name)
		}
	})

	t.Run("keeps the episode numbers the filename gives", func(t *testing.T) {
		fixture := newFixture(t)
		ctx := context.Background()

		record, err := fixture.service.SaveScanned(ctx, Scanned{
			LibraryID:         fixture.libraryID,
			Kind:              itemmodal.KindEpisode,
			Key:               "episode:the-wire:1:3",
			Name:              "The Wire S01E03",
			SortName:          "the wire s01e03",
			IndexNumber:       number(3),
			ParentIndexNumber: number(1),
			DateModified:      time.Now(),
		})
		if err != nil {
			t.Fatalf("failed to scan the episode: %v", err)
		}

		if _, err := fixture.service.UpdateMetadata(ctx, record.ID, Metadata{
			Name:        ptr("The Buys"),
			IndexNumber: number(9),
			ProviderIds: &map[string]string{"Tmdb": "62098"},
		}); err != nil {
			t.Fatalf("failed to identify the episode: %v", err)
		}

		rescanned, err := fixture.service.SaveScanned(ctx, Scanned{
			LibraryID:         fixture.libraryID,
			Kind:              itemmodal.KindEpisode,
			Key:               "episode:the-wire:1:3",
			Name:              "The Wire S01E03",
			SortName:          "the wire s01e03",
			IndexNumber:       number(3),
			ParentIndexNumber: number(1),
			DateModified:      time.Now(),
		})
		if err != nil {
			t.Fatalf("failed to rescan the episode: %v", err)
		}

		if rescanned.Name != "The Buys" {
			t.Errorf("name = %q, want the identified title", rescanned.Name)
		}
		if rescanned.IndexNumber == nil || *rescanned.IndexNumber != 3 {
			t.Errorf("index number = %v, want the filename's episode number", year(rescanned.IndexNumber))
		}
	})
}

func TestService_EditMetadata(t *testing.T) {
	fixture := newFixture(t)

	t.Run("claims a retitled item", func(t *testing.T) {
		id := fixture.add(t, seed{kind: itemmodal.KindMovie, name: "Untitled"})

		edited := fixture.editing(t, id, Metadata{Name: ptr("Named")})
		if !slices.Contains(edited.LockedFields, LockedName) {
			t.Errorf("locked fields = %v, want the title claimed", edited.LockedFields)
		}
	})

	t.Run("leaves an untouched title unclaimed", func(t *testing.T) {
		id := fixture.add(t, seed{kind: itemmodal.KindMovie, name: "Unchanged"})

		edited := fixture.editing(t, id, Metadata{
			Name:     ptr("Unchanged"),
			SortName: ptr("Unchanged"),
			Overview: ptr("A synopsis nobody wrote before."),
		})
		if slices.Contains(edited.LockedFields, LockedName) {
			t.Errorf("locked fields = %v, want the title left scanner owned", edited.LockedFields)
		}
	})

	t.Run("claims an item whose year changed", func(t *testing.T) {
		id := fixture.add(t, seed{kind: itemmodal.KindMovie, name: "Dated"})

		edited := fixture.editing(t, id, Metadata{ProductionYear: number(1999)})
		if !slices.Contains(edited.LockedFields, LockedName) {
			t.Errorf("locked fields = %v, want the title claimed", edited.LockedFields)
		}
	})

	t.Run("keeps the locks the client sent", func(t *testing.T) {
		id := fixture.add(t, seed{kind: itemmodal.KindMovie, name: "Locked"})

		edited := fixture.editing(t, id, Metadata{
			Name:         ptr("Relocked"),
			LockedFields: &[]string{"Overview"},
		})
		if want := []string{"Overview", LockedName}; !slices.Equal(edited.LockedFields, want) {
			t.Errorf("locked fields = %v, want %v", edited.LockedFields, want)
		}
	})
}

func (f *fixture) editing(t *testing.T, id uuid.UUID, metadata Metadata) *Item {
	t.Helper()

	item, err := f.service.ItemByID(context.Background(), id)
	if err != nil {
		t.Fatalf("failed to read the item: %v", err)
	}

	edited, err := f.service.EditMetadata(context.Background(), item, metadata)
	if err != nil {
		t.Fatalf("failed to edit the item: %v", err)
	}

	return edited
}

func ptr[T any](value T) *T {
	return &value
}

func year(value *int32) any {
	if value == nil {
		return nil
	}

	return *value
}
