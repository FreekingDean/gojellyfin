package tmdb

import (
	"context"
	"fmt"

	"github.com/FreekingDean/gojellyfin/internal/items"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
)

func (c *Client) Search(ctx context.Context, kind items.Kind, term string) ([]items.Metadata, error) {
	if kind != itemmodel.KindMovie && kind != itemmodel.KindSeries {
		return nil, fmt.Errorf("tmdb: nothing is searched for as %s", kind)
	}
	if err := c.ready(ctx); err != nil {
		return nil, err
	}

	found, err := c.matching(kind, term)
	if err != nil {
		return nil, err
	}

	base := c.images(ctx)
	titles := make([]items.Metadata, 0, len(found))
	for _, title := range found {
		titles = append(titles, summary(base, title))
	}

	return titles, nil
}

func (c *Client) matching(kind items.Kind, term string) ([]listed, error) {
	if kind == itemmodel.KindSeries {
		shows, err := c.api.GetSearchTVShow(term, nil)
		if err != nil {
			return nil, err
		}

		found := make([]listed, 0, len(shows.Results))
		for _, result := range shows.Results {
			found = append(found, listed{
				id:       result.ID,
				name:     result.Name,
				overview: result.Overview,
				released: result.FirstAirDate,
				score:    result.VoteAverage,
				poster:   result.PosterPath,
				backdrop: result.BackdropPath,
			})
		}

		return found, nil
	}

	movies, err := c.api.GetSearchMovies(term, nil)
	if err != nil {
		return nil, err
	}

	found := make([]listed, 0, len(movies.Results))
	for _, result := range movies.Results {
		found = append(found, listed{
			id:       result.ID,
			name:     result.Title,
			overview: result.Overview,
			released: result.ReleaseDate,
			score:    result.VoteAverage,
			poster:   result.PosterPath,
			backdrop: result.BackdropPath,
		})
	}

	return found, nil
}
