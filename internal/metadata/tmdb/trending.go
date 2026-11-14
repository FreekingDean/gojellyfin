package tmdb

import (
	"context"
	"fmt"

	"github.com/FreekingDean/gojellyfin/internal/items"
	imagemodel "github.com/FreekingDean/gojellyfin/internal/store/image"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
)

const trendingWindow = "week"

var trendingTypes = map[items.Kind]string{
	itemmodel.KindMovie:  "movie",
	itemmodel.KindSeries: "tv",
}

func (c *Client) Trending(ctx context.Context, kind items.Kind) ([]items.Metadata, error) {
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
		premiere := date(result.ReleaseDate + result.FirstAirDate)
		titles = append(titles, items.Metadata{
			Name:            text(result.Title + result.Name),
			Overview:        text(result.Overview),
			CommunityRating: score(result.VoteAverage),
			PremiereDate:    premiere,
			ProductionYear:  year(premiere),
			ProviderIds:     providerIDs(result.ID, ""),
			Images: artwork(
				remote(imagemodel.KindPrimary, base, posterSize, result.PosterPath),
				remote(imagemodel.KindBackdrop, base, backdropSize, result.BackdropPath),
			),
		})
	}

	return titles, nil
}
