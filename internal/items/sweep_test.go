package items

import (
	"context"
	"testing"

	"github.com/google/uuid"

	imagemodal "github.com/FreekingDean/gojellyfin/internal/store/image"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
)

func TestService_SweepUnreachable(t *testing.T) {
	t.Run("keeps a title a downloader still holds a file for", func(t *testing.T) {
		fixture := newFixture(t)
		ctx := context.Background()

		kept := fixture.scannedFrom(t, fixture.downloader(t), "movie:kept", "/media/kept.mkv")

		if err := fixture.service.SweepUnreachable(ctx); err != nil {
			t.Fatalf("failed to sweep: %v", err)
		}

		if _, err := fixture.service.ItemByID(ctx, kept.ID); err != nil {
			t.Errorf("a title with a file was swept: %v", err)
		}
	})

	t.Run("marks a title deleted rather than removing it, so watch state survives", func(t *testing.T) {
		fixture := newFixture(t)
		ctx := context.Background()

		gone := fixture.add(t, seed{kind: itemmodal.KindMovie, name: "Gone"})

		if err := fixture.service.SweepUnreachable(ctx); err != nil {
			t.Fatalf("failed to sweep: %v", err)
		}

		if _, err := fixture.service.ItemByID(ctx, gone); err == nil {
			t.Error("a title with no file survived the sweep")
		}

		record, err := fixture.service.store.Item.Get(ctx, gone)
		if err != nil {
			t.Fatalf("the row itself was deleted, taking its watch state: %v", err)
		}
		if record.DeletedAt == nil {
			t.Error("the row was not marked deleted")
		}
	})

	t.Run("sweeps a season and its series once their episodes have gone", func(t *testing.T) {
		fixture := newFixture(t)
		ctx := context.Background()

		series := fixture.add(t, seed{kind: itemmodal.KindSeries, name: "Show"})
		season := fixture.add(t, seed{kind: itemmodal.KindSeason, name: "Season 1", parentID: &series})
		episode := fixture.add(t, seed{kind: itemmodal.KindEpisode, name: "One", parentID: &season})

		if err := fixture.service.SweepUnreachable(ctx); err != nil {
			t.Fatalf("failed to sweep: %v", err)
		}

		for name, id := range map[string]uuid.UUID{"episode": episode, "season": season, "series": series} {
			if _, err := fixture.service.ItemByID(ctx, id); err == nil {
				t.Errorf("the %s survived with nothing beneath it", name)
			}
		}
	})

	t.Run("keeps a series while one episode still has a file", func(t *testing.T) {
		fixture := newFixture(t)
		ctx := context.Background()

		series := fixture.add(t, seed{kind: itemmodal.KindSeries, name: "Held"})
		season := fixture.add(t, seed{kind: itemmodal.KindSeason, name: "Season 1", parentID: &series})
		episode := fixture.add(t, seed{kind: itemmodal.KindEpisode, name: "One", parentID: &season})

		if _, err := fixture.service.SaveSource(ctx, ScannedSource{
			SourceID: fixture.downloader(t),
			ItemID:   episode,
			Path:     "/media/held.mkv",
			Name:     "held.mkv",
		}); err != nil {
			t.Fatalf("failed to save the file: %v", err)
		}

		if err := fixture.service.SweepUnreachable(ctx); err != nil {
			t.Fatalf("failed to sweep: %v", err)
		}

		for name, id := range map[string]uuid.UUID{"episode": episode, "season": season, "series": series} {
			if _, err := fixture.service.ItemByID(ctx, id); err != nil {
				t.Errorf("the %s was swept while a file remained: %v", name, err)
			}
		}
	})

	t.Run("leaves the artwork attached for the title's return", func(t *testing.T) {
		fixture := newFixture(t)
		ctx := context.Background()

		gone := fixture.add(t, seed{kind: itemmodal.KindMovie, name: "Returning"})
		if err := fixture.service.SaveDownloadedImage(ctx, gone, Artwork{
			Kind: imagemodal.KindPrimary, Path: "/artwork/poster.jpg", Tag: "tag",
		}); err != nil {
			t.Fatalf("failed to save the image: %v", err)
		}

		if err := fixture.service.SweepUnreachable(ctx); err != nil {
			t.Fatalf("failed to sweep: %v", err)
		}

		images, err := fixture.service.Images(ctx, gone)
		if err != nil {
			t.Fatalf("failed to query the images: %v", err)
		}
		if len(images) != 1 {
			t.Errorf("images = %d, want the artwork kept for the title's return", len(images))
		}
	})
}
