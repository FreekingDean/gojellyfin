package tracing

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"
)

func TestNew_ExportsToTheTracesPath(t *testing.T) {
	var mu sync.Mutex
	var paths []string
	collector := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		paths = append(paths, r.URL.Path)
	}))
	t.Cleanup(collector.Close)

	for endpoint, want := range map[string]string{
		collector.URL:                "/v1/traces",
		collector.URL + "/":          "/v1/traces",
		collector.URL + "/v1/traces": "/v1/traces",
		collector.URL + "/otlp":      "/otlp/v1/traces",
	} {
		mu.Lock()
		paths = nil
		mu.Unlock()
		tracing, err := New(configured(endpoint))
		if err != nil {
			t.Fatal(err)
		}
		_, span := tracing.StartRequest(context.Background(), http.Header{}, "GetPingSystem", http.MethodGet)
		span.End()
		if err := tracing.Stop(context.Background()); err != nil {
			t.Fatal(err)
		}

		mu.Lock()
		if !slices.Equal(paths, []string{want}) {
			t.Errorf("%s exported to %v, want %s", endpoint, paths, want)
		}
		mu.Unlock()
	}
}
