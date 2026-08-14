package items

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
)

func TestDeleteItemsNotInPathsKeepsTheRow(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()

	id := fixture.add(t, seed{kind: itemmodal.KindMovie, name: "Gone"})

	if err := fixture.service.DeleteItemsNotInPaths(ctx, fixture.libraryID, []string{"/somewhere/else.mkv"}); err != nil {
		t.Fatalf("failed to sweep: %v", err)
	}

	if _, err := fixture.service.ItemByID(ctx, id); err == nil {
		t.Error("a swept item is still readable")
	}

	// The row is what the watch state hangs off, so losing it loses the history.
	record, err := fixture.service.store.Item.Get(ctx, id)
	if err != nil {
		t.Fatalf("the row was deleted rather than marked: %v", err)
	}
	if record.DeletedAt == nil {
		t.Error("the row survived but was not marked deleted")
	}
}

func TestScanRevivesASweptItem(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()

	first, err := fixture.service.SaveScanned(ctx, Scanned{
		LibraryID:    fixture.libraryID,
		Kind:         itemmodal.KindMovie,
		Name:         "Returns",
		SortName:     "Returns",
		Path:         "/media/returns.mkv",
		DateModified: time.Now(),
	})
	if err != nil {
		t.Fatalf("failed to save the item: %v", err)
	}

	if err := fixture.service.DeleteItemsNotInPaths(ctx, fixture.libraryID, []string{"/somewhere/else.mkv"}); err != nil {
		t.Fatalf("failed to sweep: %v", err)
	}

	second, err := fixture.service.SaveScanned(ctx, Scanned{
		LibraryID:    fixture.libraryID,
		Kind:         itemmodal.KindMovie,
		Name:         "Returns",
		SortName:     "Returns",
		Path:         "/media/returns.mkv",
		DateModified: time.Now(),
	})
	if err != nil {
		t.Fatalf("failed to save the returning item: %v", err)
	}

	// A new row would orphan every reference the old one carried.
	if second.ID != first.ID {
		t.Errorf("the file came back as a new item: %s, was %s", second.ID, first.ID)
	}
	if second.DeletedAt != nil {
		t.Error("the returning item is still marked deleted")
	}
	if _, err := fixture.service.ItemByID(ctx, first.ID); err != nil {
		t.Errorf("the revived item is not readable: %v", err)
	}
}

// The foreign key took descendants on a hard delete and cannot on an update.
func TestDeleteItemsNotInPathsSweepsDescendants(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()

	series := fixture.add(t, seed{kind: itemmodal.KindSeries, name: "Series"})
	season := fixture.add(t, seed{kind: itemmodal.KindSeason, name: "Season 1", parentID: &series, index: number(1)})
	episode := fixture.add(t, seed{kind: itemmodal.KindEpisode, name: "S01E01", parentID: &season, index: number(1), parentIndex: number(1)})

	kept := make([]string, 0, 2)
	for _, id := range []uuid.UUID{season, episode} {
		record, err := fixture.service.store.Item.Get(ctx, id)
		if err != nil {
			t.Fatalf("failed to load the item: %v", err)
		}
		kept = append(kept, record.Path)
	}

	if err := fixture.service.DeleteItemsNotInPaths(ctx, fixture.libraryID, kept); err != nil {
		t.Fatalf("failed to sweep: %v", err)
	}

	records, total, err := fixture.service.QueryItems(ctx, ItemQuery{LibraryID: &fixture.libraryID})
	if err != nil {
		t.Fatalf("failed to query the items: %v", err)
	}
	if total != 0 || len(records) != 0 {
		t.Errorf("items = %v, want none: a season kept by path outlived its series", names(records))
	}
}
