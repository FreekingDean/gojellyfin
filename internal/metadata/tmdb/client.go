package tmdb

import (
	"context"
	"errors"
	"log"
	"net/http"
	"sync"
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

	mutex     sync.Mutex
	imageBase string
	described bool
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

func (c *Client) Movie(ctx context.Context, tmdbID int) (items.Metadata, bool, error) {
	if err := c.ready(ctx); err != nil {
		return items.Metadata{}, false, err
	}

	movie, err := c.api.GetMovieDetails(tmdbID, map[string]string{"append_to_response": "release_dates,credits"})
	if err != nil {
		return missed(err)
	}

	return movieMetadata(movie, c.images(ctx)), true, nil
}

func (c *Client) Series(ctx context.Context, tmdbID int) (items.Metadata, bool, error) {
	if err := c.ready(ctx); err != nil {
		return items.Metadata{}, false, err
	}

	series, err := c.api.GetTVDetails(tmdbID, map[string]string{"append_to_response": "content_ratings,external_ids,credits"})
	if err != nil {
		return missed(err)
	}

	return seriesMetadata(series, c.images(ctx)), true, nil
}

func (c *Client) Season(ctx context.Context, tmdbID int, season int32) (items.Metadata, bool, error) {
	if err := c.ready(ctx); err != nil {
		return items.Metadata{}, false, err
	}

	found, err := c.api.GetTVSeasonDetails(tmdbID, int(season), nil)
	if err != nil {
		return missed(err)
	}

	return seasonMetadata(found, c.images(ctx)), true, nil
}

func (c *Client) Episode(ctx context.Context, tmdbID int, season, episode int32) (items.Metadata, bool, error) {
	if err := c.ready(ctx); err != nil {
		return items.Metadata{}, false, err
	}

	found, err := c.api.GetTVEpisodeDetails(tmdbID, int(season), int(episode), map[string]string{"append_to_response": "external_ids"})
	if err != nil {
		return missed(err)
	}

	return episodeMetadata(found, c.images(ctx)), true, nil
}

func (c *Client) ready(ctx context.Context) error {
	if !c.Enabled() {
		return ErrNotConfigured
	}

	return c.wait(ctx)
}

func (c *Client) images(ctx context.Context) string {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.described {
		return c.imageBase
	}
	if err := c.wait(ctx); err != nil {
		return ""
	}

	described, err := c.api.GetConfigurationAPI()
	if err != nil {
		log.Printf("tmdb: no artwork this run, the configuration could not be read: %v", err)

		return ""
	}

	c.imageBase = described.Images.SecureBaseURL
	c.described = true

	return c.imageBase
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
