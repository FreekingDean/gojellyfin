package tmdb

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRetrying_RoundTrip(t *testing.T) {
	t.Run("backs off on too many requests", func(t *testing.T) {
		attempts := 0
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			attempts++
			if attempts < 3 {
				writer.WriteHeader(http.StatusTooManyRequests)

				return
			}
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(details[request.URL.Path]))
		}))
		defer server.Close()

		client, err := newClient(server.URL+"/3", "test-key")
		if err != nil {
			t.Fatalf("failed to build the client: %v", err)
		}
		client.api.SetClientConfig(http.Client{
			Timeout:   requestTimeout,
			Transport: retrying{base: http.DefaultTransport, attempts: retryAttempts, delay: time.Millisecond},
		})

		found, matched, err := client.Episode(context.Background(), map[string]string{providerTmdb: "1396"}, 1, 1)
		if err != nil {
			t.Fatalf("a refused request failed the run instead of backing off: %v", err)
		}
		if !matched {
			t.Fatal("the retry did not return the episode")
		}
		if found.Name == nil || *found.Name != "Pilot" {
			t.Errorf("Name = %v, want Pilot", found.Name)
		}
		if attempts != 3 {
			t.Errorf("attempts = %d, want the two refusals retried", attempts)
		}
	})

	t.Run("gives up after too many refusals", func(t *testing.T) {
		attempts := 0
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			attempts++
			writer.Header().Set("Retry-After", "0")
			writer.WriteHeader(http.StatusTooManyRequests)
		}))
		defer server.Close()

		client, err := newClient(server.URL+"/3", "test-key")
		if err != nil {
			t.Fatalf("failed to build the client: %v", err)
		}
		client.api.SetClientConfig(http.Client{
			Timeout:   requestTimeout,
			Transport: retrying{base: http.DefaultTransport, attempts: retryAttempts, delay: time.Millisecond},
		})

		if _, _, err := client.Episode(context.Background(), map[string]string{providerTmdb: "1396"}, 1, 1); err == nil {
			t.Fatal("a permanently refused request answered without an error")
		}
		if attempts != retryAttempts {
			t.Errorf("attempts = %d, want %d", attempts, retryAttempts)
		}
	})
}

func TestLimiter_wait(t *testing.T) {
	t.Run("spaces the requests it lets through", func(t *testing.T) {
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
	})

	t.Run("stops on a cancelled context", func(t *testing.T) {
		limiter := newLimiter(time.Hour)
		ctx, cancel := context.WithCancel(context.Background())

		if err := limiter.wait(ctx); err != nil {
			t.Fatalf("the first wait should not have been delayed: %v", err)
		}

		cancel()

		if err := limiter.wait(ctx); err == nil {
			t.Error("a cancelled run kept waiting on the limiter")
		}
	})
}
