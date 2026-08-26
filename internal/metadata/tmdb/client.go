package tmdb

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	gotmdb "github.com/cyruzin/golang-tmdb"

	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/items"
)

var ErrNotConfigured = errors.New("tmdb: TMDB_API_KEY is not set")

const (
	requestSpacing = 50 * time.Millisecond
	requestTimeout = 30 * time.Second
	notFound       = 34
)

type Client struct {
	api     *gotmdb.Client
	limiter *limiter
}

func NewClient(config env.Config) (*Client, error) {
	return newClient("", config.TMDB.APIKey)
}

func newClient(baseURL, apiKey string) (*Client, error) {
	client := &Client{limiter: newLimiter(requestSpacing)}
	if apiKey == "" {
		return client, nil
	}

	api, err := gotmdb.InitV4(apiKey)
	if err != nil {
		return nil, err
	}
	api.SetClientConfig(http.Client{
		Timeout:   requestTimeout,
		Transport: retrying{base: http.DefaultTransport, attempts: retryAttempts, delay: retryDelay},
	})
	if baseURL != "" {
		api.SetCustomBaseURL(baseURL)
	}

	client.api = api

	return client, nil
}

func (c *Client) Enabled() bool {
	return c.api != nil
}

func (c *Client) Movie(ctx context.Context, name string, year *int32) (items.Metadata, bool, error) {
	found, err := c.search(ctx, name, year, "year", func(query string, options map[string]string) (int64, error) {
		results, err := c.api.GetSearchMovies(query, options)
		if err != nil {
			return 0, err
		}
		if results.SearchMoviesResults == nil || len(results.Results) == 0 {
			return 0, nil
		}

		return results.Results[0].ID, nil
	})
	if err != nil || found == 0 {
		return items.Metadata{}, false, err
	}

	if err := c.wait(ctx); err != nil {
		return items.Metadata{}, false, err
	}

	movie, err := c.api.GetMovieDetails(int(found), map[string]string{"append_to_response": "release_dates"})
	if err != nil {
		return missed(err)
	}

	return movieMetadata(movie), true, nil
}

func (c *Client) Series(ctx context.Context, name string, year *int32) (items.Metadata, bool, error) {
	found, err := c.search(ctx, name, year, "first_air_date_year", func(query string, options map[string]string) (int64, error) {
		results, err := c.api.GetSearchTVShow(query, options)
		if err != nil {
			return 0, err
		}
		if results.SearchTVShowsResults == nil || len(results.Results) == 0 {
			return 0, nil
		}

		return results.Results[0].ID, nil
	})
	if err != nil || found == 0 {
		return items.Metadata{}, false, err
	}

	if err := c.wait(ctx); err != nil {
		return items.Metadata{}, false, err
	}

	series, err := c.api.GetTVDetails(int(found), map[string]string{"append_to_response": "content_ratings,external_ids"})
	if err != nil {
		return missed(err)
	}

	return seriesMetadata(series), true, nil
}

func (c *Client) Season(ctx context.Context, series map[string]string, season int32) (items.Metadata, bool, error) {
	id, err := c.seriesID(ctx, series)
	if err != nil || id == 0 {
		return items.Metadata{}, false, err
	}

	found, err := c.api.GetTVSeasonDetails(id, int(season), nil)
	if err != nil {
		return missed(err)
	}

	return seasonMetadata(found), true, nil
}

func (c *Client) Episode(ctx context.Context, series map[string]string, season, episode int32) (items.Metadata, bool, error) {
	id, err := c.seriesID(ctx, series)
	if err != nil || id == 0 {
		return items.Metadata{}, false, err
	}

	found, err := c.api.GetTVEpisodeDetails(id, int(season), int(episode), map[string]string{"append_to_response": "external_ids"})
	if err != nil {
		return missed(err)
	}

	return episodeMetadata(found), true, nil
}

func (c *Client) seriesID(ctx context.Context, series map[string]string) (int, error) {
	if !c.Enabled() {
		return 0, ErrNotConfigured
	}

	id, err := strconv.Atoi(series[providerTmdb])
	if err != nil {
		return 0, nil
	}

	return id, c.wait(ctx)
}

func (c *Client) search(
	ctx context.Context,
	name string,
	year *int32,
	yearOption string,
	run func(string, map[string]string) (int64, error),
) (int64, error) {
	if !c.Enabled() {
		return 0, ErrNotConfigured
	}
	if err := c.wait(ctx); err != nil {
		return 0, err
	}

	options := map[string]string{}
	if year != nil {
		options[yearOption] = strconv.FormatInt(int64(*year), 10)
	}

	found, err := run(name, options)
	if isNotFound(err) {
		return 0, nil
	}

	return found, err
}

func (c *Client) wait(ctx context.Context) error {
	return c.limiter.wait(ctx)
}

func missed(err error) (items.Metadata, bool, error) {
	if isNotFound(err) {
		return items.Metadata{}, false, nil
	}

	return items.Metadata{}, false, err
}

func isNotFound(err error) bool {
	var answer gotmdb.Error

	return errors.As(err, &answer) && answer.StatusCode == notFound
}
