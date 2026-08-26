package tmdb

import (
	"io"
	"net/http"
	"strconv"
	"time"
)

const (
	retryAttempts = 4
	retryDelay    = time.Second
	retryDelayMax = 30 * time.Second
)

type retrying struct {
	base     http.RoundTripper
	attempts int
	delay    time.Duration
}

func (r retrying) RoundTrip(request *http.Request) (*http.Response, error) {
	for attempt := range r.attempts {
		answer, err := r.base.RoundTrip(request)
		if err != nil {
			return nil, err
		}
		if !worthRetrying(answer.StatusCode) || attempt == r.attempts-1 {
			return answer, nil
		}

		wait := backoff(r.delay, attempt, retryAfter(answer.Header))
		_, _ = io.Copy(io.Discard, answer.Body)
		_ = answer.Body.Close()

		timer := time.NewTimer(wait)
		select {
		case <-request.Context().Done():
			timer.Stop()

			return nil, request.Context().Err()
		case <-timer.C:
		}
	}

	return r.base.RoundTrip(request)
}

func worthRetrying(status int) bool {
	return status == http.StatusTooManyRequests || status >= http.StatusInternalServerError
}

func backoff(base time.Duration, attempt int, asked time.Duration) time.Duration {
	waited := base << attempt
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
