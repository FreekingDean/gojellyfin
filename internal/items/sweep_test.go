package items

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	imagemodal "github.com/FreekingDean/gojellyfin/internal/store/image"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
)

func TestService_DeleteItemsNotInKeys(t *testing.T) {
	t.Run("refuses an empty scan", func(t *testing.T) {
		fixture := newFixture(t)
		ctx := context.Background()

		kept := fixture.add(t, seed{kind: itemmodal.KindMovie, name: "Kept"})

		err := fixture.service.DeleteItemsNotInKeys(ctx, fixture.libraryID, nil)
		if !errors.Is(err, ErrNothingScanned) {
			t.Fatalf("got %v, want ErrNothingScanned", err)
		}

		if _, err := fixture.service.ItemByID(ctx, kept); err != nil {
			t.Errorf("an empty scan deleted the item: %v", err)
		}
	})

	t.Run("keeps the row and marks it deleted", func(t *testing.T) {
		fixture := newFixture(t)
		ctx := context.Background()

		id := fixture.add(t, seed{kind: itemmodal.KindMovie, name: "Gone"})

		if err := fixture.service.DeleteItemsNotInKeys(ctx, fixture.libraryID, []string{"movie:elsewhere"}); err != nil {
			t.Fatalf("failed to sweep: %v", err)
		}

		if _, err := fixture.service.ItemByID(ctx, id); err == nil {
			t.Error("a swept item is still readable")
		}

		record, err := fixture.service.store.Item.Get(ctx, id)
		if err != nil {
			t.Fatalf("the row was deleted rather than marked: %v", err)
		}
		if record.DeletedAt == nil {
			t.Error("the row survived but was not marked deleted")
		}
	})

	t.Run("sweeps descendants", func(t *testing.T) {
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
			kept = append(kept, record.Key)
		}

		if err := fixture.service.DeleteItemsNotInKeys(ctx, fixture.libraryID, kept); err != nil {
			t.Fatalf("failed to sweep: %v", err)
		}

		records, total, err := fixture.service.QueryItems(ctx, ItemQuery{LibraryID: &fixture.libraryID})
		if err != nil {
			t.Fatalf("failed to query the items: %v", err)
		}
		if total != 0 || len(records) != 0 {
			t.Errorf("items = %v, want none: a season kept by key outlived its series", names(records))
		}
	})

	t.Run("takes descendants with it", func(t *testing.T) {
		fixture := newFixture(t)
		ctx := context.Background()

		series := fixture.add(t, seed{kind: itemmodal.KindSeries, name: "Series"})
		season := fixture.add(t, seed{kind: itemmodal.KindSeason, name: "Season 1", parentID: &series, index: number(1)})
		episode := fixture.add(t, seed{kind: itemmodal.KindEpisode, name: "S01E01", parentID: &season, index: number(1), parentIndex: number(1)})

		kept := make([]string, 0, 2)
		for _, id := range []uuid.UUID{season, episode} {
			record, err := fixture.service.ItemByID(ctx, id)
			if err != nil {
				t.Fatalf("failed to load the item: %v", err)
			}
			kept = append(kept, record.Key)
		}

		if err := fixture.service.DeleteItemsNotInKeys(ctx, fixture.libraryID, kept); err != nil {
			t.Fatalf("failed to prune the items: %v", err)
		}

		records, total, err := fixture.service.QueryItems(ctx, ItemQuery{LibraryID: &fixture.libraryID})
		if err != nil {
			t.Fatalf("failed to query the items: %v", err)
		}
		if total != 0 || len(records) != 0 {
			t.Errorf("items = %v, want none", names(records))
		}
	})

	t.Run("takes the images with it", func(t *testing.T) {
		fixture := newFixture(t)
		ctx := context.Background()

		kept := fixture.add(t, seed{kind: itemmodal.KindMovie, name: "Kept"})
		pruned := fixture.add(t, seed{kind: itemmodal.KindMovie, name: "Pruned"})

		artwork := []Artwork{{Kind: imagemodal.KindPrimary, Path: "/artwork/poster.jpg", Tag: "tag"}}
		if err := fixture.service.ReplaceImages(ctx, pruned, artwork); err != nil {
			t.Fatalf("failed to save the images: %v", err)
		}

		survivor, err := fixture.service.ItemByID(ctx, kept)
		if err != nil {
			t.Fatalf("failed to load the kept item: %v", err)
		}

		if err := fixture.service.DeleteItemsNotInKeys(ctx, fixture.libraryID, []string{survivor.Key}); err != nil {
			t.Fatalf("failed to prune the items: %v", err)
		}

		records, total, err := fixture.service.QueryItems(ctx, ItemQuery{LibraryID: &fixture.libraryID})
		if err != nil {
			t.Fatalf("failed to query the items: %v", err)
		}
		if total != 1 || len(records) != 1 || records[0].ID != kept {
			t.Errorf("items = %v, want only the kept item", names(records))
		}

		images, err := fixture.service.Images(ctx, pruned)
		if err != nil {
			t.Fatalf("failed to query the images: %v", err)
		}
		if len(images) != 1 {
			t.Errorf("images = %d, want the artwork kept for the item's return", len(images))
		}
	})
}
