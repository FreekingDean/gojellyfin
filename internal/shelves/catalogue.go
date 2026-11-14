package shelves

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
)

const (
	hiddenGemRating    = 7.5
	shortOnTimeMinutes = 100
	ticksPerMinute     = int64(time.Minute / 100)
)

var Defaults = []string{"recently-aired", "new-releases"}

var definitions = []definition{
	fixed("recently-aired", "Recently Aired", libraries.CollectionTypeTvshows, items.LatestQuery{
		Kinds:      episodes,
		Order:      items.LatestAired,
		NewestOnly: true,
	}),
	fixed("new-releases", "New Releases", libraries.CollectionTypeMovies, items.LatestQuery{
		Kinds: movies,
		Order: items.LatestAired,
	}),
	unwatched("hidden-gems", "Hidden Gems", libraries.CollectionTypeMovies, items.LatestQuery{
		Kinds:     movies,
		MinRating: ptr(hiddenGemRating),
		Order:     items.LatestRated,
	}),
	unwatched("short-on-time", "Short on Time", libraries.CollectionTypeMovies, items.LatestQuery{
		Kinds:      movies,
		MaxRunTime: ptr(shortOnTimeMinutes * ticksPerMinute),
		Order:      items.LatestRated,
	}),
	seasonal("spooky-season", "Spooky Season", time.October, items.LatestQuery{
		Kinds:  movies,
		Genres: []string{"Horror"},
		Order:  items.LatestRated,
	}),
	{slug: "genre-spotlight", collection: libraries.CollectionTypeMovies, resolve: genreSpotlight},
	{slug: "because-you-watched", collection: libraries.CollectionTypeMovies, resolve: becauseYouWatched},
	trending("trending-movies", "Trending Movies", libraries.CollectionTypeMovies, itemmodel.KindMovie),
	trending("trending-shows", "Trending Shows", libraries.CollectionTypeTvshows, itemmodel.KindSeries),
}

var (
	movies   = []items.Kind{itemmodel.KindMovie}
	episodes = []items.Kind{itemmodel.KindEpisode}
)

type resolver func(ctx context.Context, catalogue Catalogue, viewer items.Viewer, userID uuid.UUID, now time.Time) (string, items.LatestQuery, bool, error)

type definition struct {
	slug       string
	title      string
	collection libraries.CollectionType
	resolve    resolver
	trending   items.Kind
	shared     bool
}

func trending(slug, name string, collection libraries.CollectionType, kind items.Kind) definition {
	return definition{slug: slug, title: name, collection: collection, trending: kind, resolve: func(context.Context, Catalogue, items.Viewer, uuid.UUID, time.Time) (string, items.LatestQuery, bool, error) {
		return name, items.LatestQuery{}, true, nil
	}}
}

func fixed(slug, name string, collection libraries.CollectionType, query items.LatestQuery) definition {
	return definition{slug: slug, title: name, collection: collection, shared: true, resolve: func(context.Context, Catalogue, items.Viewer, uuid.UUID, time.Time) (string, items.LatestQuery, bool, error) {
		return name, query, true, nil
	}}
}

func unwatched(slug, name string, collection libraries.CollectionType, query items.LatestQuery) definition {
	return definition{slug: slug, title: name, collection: collection, resolve: func(_ context.Context, _ Catalogue, _ items.Viewer, userID uuid.UUID, _ time.Time) (string, items.LatestQuery, bool, error) {
		query.UnplayedBy = userID
		return name, query, true, nil
	}}
}

func seasonal(slug, name string, month time.Month, query items.LatestQuery) definition {
	return definition{slug: slug, title: name, collection: libraries.CollectionTypeMovies, shared: true, resolve: func(_ context.Context, _ Catalogue, _ items.Viewer, _ uuid.UUID, now time.Time) (string, items.LatestQuery, bool, error) {
		return name, query, now.Month() == month, nil
	}}
}

func genreSpotlight(ctx context.Context, catalogue Catalogue, viewer items.Viewer, _ uuid.UUID, now time.Time) (string, items.LatestQuery, bool, error) {
	genres, _, err := catalogue.DistinctGenres(ctx, items.MetadataQuery{Viewer: viewer, Kinds: movies})
	if err != nil || len(genres) == 0 {
		return "", items.LatestQuery{}, false, err
	}

	_, week := now.ISOWeek()
	genre := genres[week%len(genres)].Name

	return "Spotlight: " + genre, items.LatestQuery{
		Kinds:  movies,
		Genres: []string{genre},
		Order:  items.LatestRated,
	}, true, nil
}

func becauseYouWatched(ctx context.Context, catalogue Catalogue, viewer items.Viewer, userID uuid.UUID, _ time.Time) (string, items.LatestQuery, bool, error) {
	watched, genres, err := catalogue.LastPlayed(ctx, viewer, userID, itemmodel.KindMovie)
	if err != nil || watched == nil || len(genres) == 0 {
		return "", items.LatestQuery{}, false, err
	}

	return "Because You Watched " + watched.Name, items.LatestQuery{
		Kinds:      movies,
		Genres:     genres,
		UnplayedBy: userID,
		Order:      items.LatestRated,
	}, true, nil
}

func ptr[T any](value T) *T {
	return &value
}
