package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/observability/tracing"
)

func recorded(t *testing.T, operationID, method, target string) *tracing.Recorder {
	t.Helper()

	traces, recorder := tracing.Recorded()
	handler := NewOapiTracing(traces).Middleware(
		func(ctx context.Context, w http.ResponseWriter, r *http.Request, request any) (any, error) {
			return nil, nil
		}, operationID)

	r := httptest.NewRequest(method, target, nil)
	if _, err := handler(r.Context(), httptest.NewRecorder(), r, nil); err != nil {
		t.Fatal(err)
	}

	return recorder
}

func TestOapiTracing_Middleware(t *testing.T) {
	t.Run("a short operation under a streaming root is traced like any other", func(t *testing.T) {
		names := recorded(t, "GetAdditionalPart", http.MethodGet, "/Videos/6f1b/AdditionalParts").Names()

		if len(names) != 1 || names[0] != "GetAdditionalPart" {
			t.Errorf("want one span named for the operation, got %v", names)
		}
	})

	t.Run("the operation id names the span", func(t *testing.T) {
		names := recorded(t, "GetItems", http.MethodGet, "/Items?userId=6f1b").Names()

		if len(names) != 1 || names[0] != "GetItems" {
			t.Errorf("want one span named for the operation, got %v", names)
		}
	})

	t.Run("nothing the client sent reaches the span", func(t *testing.T) {
		values := recorded(t, "GetItems", http.MethodGet, "/Items?api_key=hunter2&searchTerm=secret").Values()

		for key, value := range values {
			if value == "hunter2" || value == "secret" {
				t.Errorf("%s carries %q", key, value)
			}
		}
	})
}
