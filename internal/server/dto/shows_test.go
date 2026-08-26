package dto

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/store"
	imagemodal "github.com/FreekingDean/gojellyfin/internal/store/image"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
)

func TestItemDtos(t *testing.T) {
	ctx := context.Background()

	config, err := env.Load()
	if err != nil {
		t.Fatalf("failed to read the environment: %v", err)
	}

	connection, err := store.NewStore(config)
	if err != nil {
		t.Fatalf("failed to open the database: %v", err)
	}
	if err := connection.Start(); err != nil {
		t.Fatalf("failed to reach the database, set DATABASE_URL: %v", err)
	}

	client := connection.Client()
	name := t.Name() + "-" + uuid.NewString()
	library, err := client.Library.Create().SetName(name).Save(ctx)
	if err != nil {
		t.Fatalf("failed to create the library: %v", err)
	}

	t.Cleanup(func() {
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

	service := items.New(client)
	save := func(kind items.Kind, itemName string, parentID *uuid.UUID) *items.Item {
		record, err := service.SaveScanned(ctx, items.Scanned{
			LibraryID: library.ID,
			ParentID:  parentID,
			Kind:      kind,
			Name:      itemName,
			SortName:  itemName,
			Key:       "test:" + itemName,
		})
		if err != nil {
			t.Fatalf("failed to save %q: %v", itemName, err)
		}

		return record
	}

	series := save(itemmodal.KindSeries, name+" Series", nil)
	season := save(itemmodal.KindSeason, name+" Season 1", &series.ID)
	episode := save(itemmodal.KindEpisode, name+" S01E01", &season.ID)
	movie := save(itemmodal.KindMovie, name+" Movie", nil)

	if err := service.ReplaceImages(ctx, series.ID, []items.Artwork{
		{Kind: imagemodal.KindPrimary, Path: "/fixtures/poster.jpg", Tag: "poster"},
	}); err != nil {
		t.Fatalf("failed to give the series an image: %v", err)
	}

	converted, err := ItemDtos(ctx, service, []*items.Item{episode, season, series, movie})
	if err != nil {
		t.Fatalf("failed to convert the items: %v", err)
	}

	byID := map[uuid.UUID]api.BaseItemDto{}
	for _, record := range converted {
		byID[*record.Id] = record
	}

	t.Run("an episode carries its season and its series", func(t *testing.T) {
		record := byID[episode.ID]
		if record.SeriesId == nil || *record.SeriesId != series.ID {
			t.Errorf("series id = %v, want %s", record.SeriesId, series.ID)
		}
		if record.SeriesName == nil || *record.SeriesName != series.Name {
			t.Errorf("series name = %v, want %s", record.SeriesName, series.Name)
		}
		if record.SeasonId == nil || *record.SeasonId != season.ID {
			t.Errorf("season id = %v, want %s", record.SeasonId, season.ID)
		}
		if record.SeasonName == nil || *record.SeasonName != season.Name {
			t.Errorf("season name = %v, want %s", record.SeasonName, season.Name)
		}
		if record.SeriesPrimaryImageTag == nil || *record.SeriesPrimaryImageTag != "poster" {
			t.Errorf("series primary image tag = %v, want poster", record.SeriesPrimaryImageTag)
		}
	})

	t.Run("a season carries its series and no season of its own", func(t *testing.T) {
		record := byID[season.ID]
		if record.SeriesId == nil || *record.SeriesId != series.ID {
			t.Errorf("series id = %v, want %s", record.SeriesId, series.ID)
		}
		if record.SeriesName == nil || *record.SeriesName != series.Name {
			t.Errorf("series name = %v, want %s", record.SeriesName, series.Name)
		}
		if record.SeasonId != nil {
			t.Errorf("season id = %v, want nothing", record.SeasonId)
		}
	})

	t.Run("a series carries no series of its own", func(t *testing.T) {
		record := byID[series.ID]
		if record.SeriesId != nil {
			t.Errorf("series id = %v, want nothing", record.SeriesId)
		}
	})

	t.Run("a movie carries nothing", func(t *testing.T) {
		record := byID[movie.ID]
		if record.SeriesId != nil || record.SeasonId != nil {
			t.Errorf("series id = %v and season id = %v, want nothing", record.SeriesId, record.SeasonId)
		}
	})
}
