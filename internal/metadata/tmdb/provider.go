package tmdb

import (
	"context"
	"errors"
	"strconv"

	"github.com/FreekingDean/gojellyfin/internal/items"
)

func (c *Client) Movie(ctx context.Context, name string, year *int32) (items.Metadata, bool, error) {
	id, err := c.searchMovie(ctx, name, year)
	if err != nil {
		return missed(err)
	}

	movie, err := c.movie(ctx, id)
	if err != nil {
		return missed(err)
	}

	return movieMetadata(movie), true, nil
}

func (c *Client) Series(ctx context.Context, name string, year *int32) (items.Metadata, bool, error) {
	id, err := c.searchSeries(ctx, name, year)
	if err != nil {
		return missed(err)
	}

	found, err := c.series(ctx, id)
	if err != nil {
		return missed(err)
	}

	return seriesMetadata(found), true, nil
}

func (c *Client) Episode(ctx context.Context, series map[string]string, season, episode int32) (items.Metadata, bool, error) {
	id, err := strconv.Atoi(series[providerTmdb])
	if err != nil {
		return items.Metadata{}, false, nil
	}

	found, err := c.episode(ctx, id, season, episode)
	if err != nil {
		return missed(err)
	}

	return episodeMetadata(found), true, nil
}

func missed(err error) (items.Metadata, bool, error) {
	if errors.Is(err, errNoMatch) {
		return items.Metadata{}, false, nil
	}

	return items.Metadata{}, false, err
}
