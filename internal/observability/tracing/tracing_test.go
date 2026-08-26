package tracing

import (
	"context"
	"net/http"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/env"
)

func configured(endpoint string) env.Config {
	return env.Config{Tracing: env.Tracing{OTLPEndpoint: endpoint}}
}

func started(t *testing.T, tracing *Tracing) *Span {
	t.Helper()

	_, span := tracing.StartRequest(context.Background(), http.Header{}, "GetItems", http.MethodGet)

	return span
}

func TestNew(t *testing.T) {
	t.Run("no endpoint records nothing and holds no provider to shut down", func(t *testing.T) {
		tracing, err := New(env.Config{})
		if err != nil {
			t.Fatalf("an unconfigured collector failed to build: %v", err)
		}

		if tracing.Enabled() {
			t.Error("tracing reports itself enabled with no collector configured")
		}

		started(t, tracing).End()

		if err := tracing.Stop(context.Background()); err != nil {
			t.Errorf("stopping an unconfigured tracer failed: %v", err)
		}
	})

	t.Run("an endpoint is enabled", func(t *testing.T) {
		tracing, err := New(configured("http://collector.example:4318"))
		if err != nil {
			t.Fatalf("failed to build: %v", err)
		}
		t.Cleanup(func() {
			if err := tracing.Stop(context.Background()); err != nil {
				t.Errorf("stopping failed: %v", err)
			}
		})

		if !tracing.Enabled() {
			t.Error("a collector is configured and tracing reports itself off")
		}
	})

	t.Run("a malformed endpoint is refused rather than silently exported to localhost", func(t *testing.T) {
		for _, endpoint := range []string{"collector:4318", "grpc://collector:4318", "http://", "not a url"} {
			t.Run(endpoint, func(t *testing.T) {
				if _, err := New(configured(endpoint)); err == nil {
					t.Errorf("%q was accepted", endpoint)
				}
			})
		}
	})
}

func TestRecorded(t *testing.T) {
	tracing, recorder := Recorded()

	started(t, tracing).End()

	names := recorder.Names()
	if len(names) != 1 || names[0] != "GetItems" {
		t.Errorf("want one span named for the operation, got %v", names)
	}
	if method := recorder.Values()["http.request.method"]; method != http.MethodGet {
		t.Errorf("http.request.method = %q, want GET", method)
	}
}
