package tmdb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"
)

// Unset means the provider is off, the way an empty TEMPORAL_HOSTPORT leaves
// background work off. We ship no key of our own, so there is none to share,
// none to throttle and no attribution owed for one.
var ErrNotConfigured = errors.New("tmdb: TMDB_API_KEY is not set")

var errNoMatch = errors.New("tmdb: nothing matched")

const (
	apiURL         = "https://api.themoviedb.org"
	requestSpacing = 50 * time.Millisecond
	requestTimeout = 30 * time.Second
	retryAttempts  = 4
	retryDelay     = time.Second
	retryDelayMax  = 30 * time.Second
)

type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
	limiter *limiter
	delay   time.Duration
}

func NewClient() *Client {
	return newClient(apiURL, os.Getenv("TMDB_API_KEY"))
}

func newClient(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		http:    &http.Client{Timeout: requestTimeout},
		limiter: newLimiter(requestSpacing),
		delay:   retryDelay,
	}
}

func (c *Client) Enabled() bool {
	return c.apiKey != ""
}

func (c *Client) SearchMovie(ctx context.Context, name string, year *int32) (int, error) {
	query := url.Values{"query": {name}}
	if year != nil {
		query.Set("year", strconv.FormatInt(int64(*year), 10))
	}

	return c.search(ctx, "/3/search/movie", query)
}

func (c *Client) SearchSeries(ctx context.Context, name string, year *int32) (int, error) {
	query := url.Values{"query": {name}}
	if year != nil {
		query.Set("first_air_date_year", strconv.FormatInt(int64(*year), 10))
	}

	return c.search(ctx, "/3/search/tv", query)
}

func (c *Client) search(ctx context.Context, path string, query url.Values) (int, error) {
	var results searchResults
	if err := c.get(ctx, path, query, &results); err != nil {
		return 0, err
	}
	if len(results.Results) == 0 {
		return 0, errNoMatch
	}

	return results.Results[0].ID, nil
}

func (c *Client) Movie(ctx context.Context, id int) (*Movie, error) {
	movie := &Movie{}
	query := url.Values{"append_to_response": {"release_dates"}}

	return movie, c.get(ctx, fmt.Sprintf("/3/movie/%d", id), query, movie)
}

func (c *Client) Series(ctx context.Context, id int) (*Series, error) {
	series := &Series{}
	query := url.Values{"append_to_response": {"content_ratings,external_ids"}}

	return series, c.get(ctx, fmt.Sprintf("/3/tv/%d", id), query, series)
}

func (c *Client) Episode(ctx context.Context, seriesID int, season, episode int32) (*Episode, error) {
	found := &Episode{}
	path := fmt.Sprintf("/3/tv/%d/season/%d/episode/%d", seriesID, season, episode)
	query := url.Values{"append_to_response": {"external_ids"}}

	return found, c.get(ctx, path, query, found)
}

func (c *Client) get(ctx context.Context, path string, query url.Values, out any) error {
	if !c.Enabled() {
		return ErrNotConfigured
	}

	query.Set("api_key", c.apiKey)
	address := c.baseURL + path + "?" + query.Encode()

	for attempt := range retryAttempts {
		if err := c.limiter.wait(ctx); err != nil {
			return err
		}

		answer, err := c.do(ctx, address)
		if err != nil {
			return err
		}

		switch {
		case answer.status == http.StatusOK:
			return json.Unmarshal(answer.body, out)
		case answer.status == http.StatusNotFound:
			return errNoMatch
		case answer.status != http.StatusTooManyRequests && answer.status < http.StatusInternalServerError:
			return fmt.Errorf("tmdb: %s answered %d", path, answer.status)
		case attempt == retryAttempts-1:
			return fmt.Errorf("tmdb: %s answered %d after %d attempts", path, answer.status, retryAttempts)
		}

		if err := sleep(ctx, c.backoff(attempt, answer.retryAfter)); err != nil {
			return err
		}
	}

	return errNoMatch
}

type response struct {
	status     int
	retryAfter time.Duration
	body       []byte
}

func (c *Client) do(ctx context.Context, address string) (response, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return response{}, err
	}
	request.Header.Set("Accept", "application/json")

	answer, err := c.http.Do(request)
	if err != nil {
		return response{}, err
	}
	defer func() { _ = answer.Body.Close() }()

	body, err := io.ReadAll(answer.Body)
	if err != nil {
		return response{}, err
	}

	return response{status: answer.StatusCode, retryAfter: retryAfter(answer.Header), body: body}, nil
}

func (c *Client) backoff(attempt int, asked time.Duration) time.Duration {
	waited := c.delay << attempt
	if asked > waited {
		waited = asked
	}
	if waited > retryDelayMax {
		return retryDelayMax
	}

	return waited
}

func retryAfter(header http.Header) time.Duration {
	seconds, err := strconv.Atoi(header.Get("Retry-After"))
	if err != nil || seconds <= 0 {
		return 0
	}

	return time.Duration(seconds) * time.Second
}

func sleep(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
