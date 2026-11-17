package dto

import (
	"context"
	"slices"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/storetest"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	imagemodel "github.com/FreekingDean/gojellyfin/internal/store/image"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
)

func TestItemDtos(t *testing.T) {
	ctx := context.Background()

	connection := storetest.Open(t)

	client := connection.Client()
	name := t.Name() + "-" + uuid.NewString()
	t.Cleanup(func() {
		if _, err := client.Item.Delete().Where(itemmodel.KeyHasPrefix("test:" + name)).Exec(context.Background()); err != nil {
			t.Errorf("failed to delete the items: %v", err)
		}
	})

	service := items.New(client)
	save := itemDtosSave(t, service, ctx)

	series := save(itemmodel.KindSeries, name+" Series", nil)
	season := save(itemmodel.KindSeason, name+" Season 1", &series.ID)
	episode := save(itemmodel.KindEpisode, name+" S01E01", &season.ID)
	movie := save(itemmodel.KindMovie, name+" Movie", nil)

	if err := service.SaveImage(ctx, series.ID, items.Image{
		Kind: imagemodel.KindPrimary, URL: "https://image.tmdb.org/t/p/w780/poster.jpg", Tag: "poster",
	}); err != nil {
		t.Fatalf("failed to give the series an image: %v", err)
	}
	if err := service.SaveImage(ctx, series.ID, items.Image{
		Kind: imagemodel.KindBackdrop, URL: "https://image.tmdb.org/t/p/w1280/backdrop.jpg", Tag: "backdrop",
	}); err != nil {
		t.Fatalf("failed to give the series a backdrop: %v", err)
	}

	itemDtosRest(t, service, ctx, series, movie, episode, season)
}

func itemDtosRest(t *testing.T, service *items.Service, ctx context.Context, series *items.Item, movie *items.Item, episode *items.Item, season *items.Item) {
	t.Helper()

	if err := service.SaveImage(ctx, series.ID, items.Image{
		Kind: imagemodel.KindThumb, URL: "https://image.tmdb.org/t/p/w780/thumb.jpg", Tag: "thumb",
	}); err != nil {
		t.Fatalf("failed to give the series a thumb: %v", err)
	}
	if err := service.SaveImage(ctx, movie.ID, items.Image{
		Kind: imagemodel.KindBackdrop, URL: "https://image.tmdb.org/t/p/w1280/movie.jpg", Tag: "movie-backdrop",
	}); err != nil {
		t.Fatalf("failed to give the movie a backdrop: %v", err)
	}

	converted, err := ItemDtos(ctx, service, []*items.Item{episode, season, series, movie})
	if err != nil {
		t.Fatalf("failed to convert the items: %v", err)
	}

	byID := map[uuid.UUID]api.BaseItemDto{}
	for _, record := range converted {
		byID[*record.Id] = record
	}

	t.Run("an episode carries its season and its series", anEpisodeCarriesItsSeasonAndItsSeries(byID, episode, series, season))

	t.Run("a season carries its series and no season of its own", aSeasonCarriesItsSeriesAndNoSeasonOfItsOwn(byID, season, series))

	t.Run("a series carries no series of its own", func(t *testing.T) {
		record := byID[series.ID]
		if record.SeriesId != nil {
			t.Errorf("series id = %v, want nothing", record.SeriesId)
		}
	})

	t.Run("a movie carries nothing", aMovieCarriesNothing(byID, movie))
}

func itemDtosSave(t *testing.T, service *items.Service, ctx context.Context) func(kind items.Kind, itemName string, parentID *uuid.UUID) *items.Item {
	t.Helper()

	return func(kind items.Kind, itemName string, parentID *uuid.UUID) *items.Item {
		record, err := service.SaveScanned(ctx, items.Item{
			ParentID: parentID,
			Kind:     kind,
			Name:     itemName,
			SortName: itemName,
			Key:      "test:" + itemName,
		})
		if err != nil {
			t.Fatalf("failed to save %q: %v", itemName, err)
		}

		return record
	}
}

func aMovieCarriesNothing(byID map[uuid.UUID]api.BaseItemDto, movie *items.Item) func(*testing.T) {
	return func(t *testing.T) {
		record := byID[movie.ID]
		if record.SeriesId != nil || record.SeasonId != nil {
			t.Errorf("series id = %v and season id = %v, want nothing", record.SeriesId, record.SeasonId)
		}
		if record.ParentBackdropItemId != nil || record.ParentThumbItemId != nil {
			t.Errorf("parent backdrop = %v and parent thumb = %v, want nothing", record.ParentBackdropItemId, record.ParentThumbItemId)
		}
		if record.BackdropImageTags == nil || !slices.Equal(*record.BackdropImageTags, []string{"movie-backdrop"}) {
			t.Errorf("backdrop image tags = %v, want its own backdrop", record.BackdropImageTags)
		}
	}
}

func aSeasonCarriesItsSeriesAndNoSeasonOfItsOwn(byID map[uuid.UUID]api.BaseItemDto, season *items.Item, series *items.Item) func(*testing.T) {
	return func(t *testing.T) {
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
		landscape(t, record, series.ID)
	}
}

func anEpisodeCarriesItsSeasonAndItsSeries(byID map[uuid.UUID]api.BaseItemDto, episode *items.Item, series *items.Item, season *items.Item) func(*testing.T) {
	return func(t *testing.T) {
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
		landscape(t, record, series.ID)
	}
}

func landscape(t *testing.T, record api.BaseItemDto, seriesID uuid.UUID) {
	t.Helper()

	if record.ParentBackdropItemId == nil || *record.ParentBackdropItemId != seriesID {
		t.Errorf("parent backdrop item id = %v, want the series %s", record.ParentBackdropItemId, seriesID)
	}
	if record.ParentBackdropImageTags == nil || !slices.Equal(*record.ParentBackdropImageTags, []string{"backdrop"}) {
		t.Errorf("parent backdrop image tags = %v, want the series backdrop", record.ParentBackdropImageTags)
	}
	if record.ParentThumbItemId == nil || *record.ParentThumbItemId != seriesID {
		t.Errorf("parent thumb item id = %v, want the series %s", record.ParentThumbItemId, seriesID)
	}
	if record.ParentThumbImageTag == nil || *record.ParentThumbImageTag != "thumb" {
		t.Errorf("parent thumb image tag = %v, want thumb", record.ParentThumbImageTag)
	}
}
