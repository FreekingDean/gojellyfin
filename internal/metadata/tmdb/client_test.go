package tmdb

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/FreekingDean/gojellyfin/internal/env"
)

func TestClientBacksOffOnTooManyRequests(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		attempts++
		if attempts < 3 {
			writer.WriteHeader(http.StatusTooManyRequests)

			return
		}
		_, _ = writer.Write([]byte(matrixDetail))
	}))
	defer server.Close()

	client := newClient(server.URL, "test-key")
	client.delay = time.Millisecond

	movie, err := client.movie(context.Background(), 603)
	if err != nil {
		t.Fatalf("a refused request failed the run instead of backing off: %v", err)
	}
	if attempts != 3 {
		t.Errorf("attempts = %d, want the two refusals retried", attempts)
	}
	if movie.IMDbID != "tt0133093" {
		t.Errorf("IMDbID = %q, want the retry to have returned the film", movie.IMDbID)
	}
}

func TestClientGivesUpAfterTooManyRefusals(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Retry-After", "0")
		writer.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	client := newClient(server.URL, "test-key")
	client.delay = time.Millisecond

	if _, err := client.movie(context.Background(), 603); err == nil {
		t.Fatal("a permanently refused request answered without an error")
	}
}

func TestClientRefusesWithoutAKey(t *testing.T) {
	client := newClient("http://127.0.0.1:0", "")

	if _, err := client.movie(context.Background(), 603); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("err = %v, want ErrNotConfigured", err)
	}
}

func TestClientIsOffWithoutAConfiguredKey(t *testing.T) {
	if NewClient(env.Config{}).Enabled() {
		t.Error("a client with no key configured reported itself enabled")
	}
	if !NewClient(env.Config{TMDB: env.TMDB{APIKey: "not-a-real-key"}}).Enabled() {
		t.Error("a configured key did not reach the client")
	}
}

func TestClientSpacesItsRequests(t *testing.T) {
	limiter := newLimiter(20 * time.Millisecond)
	started := time.Now()

	for range 3 {
		if err := limiter.wait(context.Background()); err != nil {
			t.Fatalf("the limiter failed: %v", err)
		}
	}

	if elapsed := time.Since(started); elapsed < 40*time.Millisecond {
		t.Errorf("three requests took %v, want them spaced", elapsed)
	}
}
