package tmdb

import (
	"context"
	"fmt"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/observability/tracing"
	imagemodel "github.com/FreekingDean/gojellyfin/internal/store/image"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
)

const trendingWindow = "week"

var trendingTypes = map[items.Kind]string{
	itemmodel.KindMovie:  "movie",
	itemmodel.KindSeries: "tv",
}

func (c *Client) Trending(ctx context.Context, kind items.Kind) ([]items.Metadata, error) {
	ctx, span := tracing.Start(ctx, "tmdb Trending")
	defer span.End()

	mediaType, known := trendingTypes[kind]
	if !known {
		return nil, fmt.Errorf("tmdb: nothing trends as %s", kind)
	}
	if err := c.ready(ctx); err != nil {
		return nil, err
	}

	trending, err := c.api.GetTrending(mediaType, trendingWindow, nil)
	if err != nil {
		return nil, err
	}

	base := c.images(ctx)
	titles := make([]items.Metadata, 0, len(trending.Results))
	for _, result := range trending.Results {
		titles = append(titles, summary(base, listed{
			id:       result.ID,
			name:     result.Title + result.Name,
			overview: result.Overview,
			released: result.ReleaseDate + result.FirstAirDate,
			score:    result.VoteAverage,
			poster:   result.PosterPath,
			backdrop: result.BackdropPath,
		}))
	}

	return titles, nil
}

type listed struct {
	id       int64
	name     string
	overview string
	released string
	score    float32
	poster   string
	backdrop string
}

func summary(base string, title listed) items.Metadata {
	premiere := date(title.released)

	return items.Metadata{
		Name:            text(title.name),
		Overview:        text(title.overview),
		CommunityRating: score(title.score),
		PremiereDate:    premiere,
		ProductionYear:  year(premiere),
		ProviderIds:     providerIDs(title.id, ""),
		Images: artwork(
			remote(imagemodel.KindPrimary, base, posterSize, title.poster),
			remote(imagemodel.KindBackdrop, base, backdropSize, title.backdrop),
		),
	}
}
