package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func recorded(t *testing.T, operationID, method, target string) []sdktrace.ReadOnlySpan {
	t.Helper()

	recorder := tracetest.NewSpanRecorder()
	tracing := &OapiTracing{
		tracer:     sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder)).Tracer("test"),
		propagator: propagation.TraceContext{},
	}

	handler := tracing.Middleware(func(ctx context.Context, w http.ResponseWriter, r *http.Request, request any) (any, error) {
		return nil, nil
	}, operationID)

	r := httptest.NewRequest(method, target, nil)
	if _, err := handler(r.Context(), httptest.NewRecorder(), r, nil); err != nil {
		t.Fatal(err)
	}

	return recorder.Ended()
}

// A span held open for the length of a transcode is a leak rather than a
// trace, so the streaming roots must produce none at all.
func TestStreamingRoutesAreNotTraced(t *testing.T) {
	paths := []string{
		"/Videos/6f1b/stream",
		"/Videos/6f1b/stream.mkv",
		"/videos/6f1b/stream",
		"/Audio/6f1b/stream",
		"/Audio/6f1b/stream.mp3",
		"/Audio/6f1b/universal",
		"/audio/6f1b/universal",
	}

	for _, path := range paths {
		if spans := recorded(t, "GetVideoStream", http.MethodGet, path); len(spans) != 0 {
			t.Errorf("%q: want no span, got %d", path, len(spans))
		}
	}
}

func TestOperationIDNamesTheSpan(t *testing.T) {
	spans := recorded(t, "GetItems", http.MethodGet, "/Items?userId=6f1b")
	if len(spans) != 1 {
		t.Fatalf("want one span, got %d", len(spans))
	}

	if spans[0].Name() != "GetItems" {
		t.Errorf("want the span named for the operation, got %q", spans[0].Name())
	}
}

// Nothing the client sent may reach a span: the query string carries api_key
// and the path carries names in some routes.
func TestSpanCarriesNoRequestDetail(t *testing.T) {
	spans := recorded(t, "GetItems", http.MethodGet, "/Items?api_key=hunter2&searchTerm=secret")

	for _, attribute := range spans[0].Attributes() {
		if value := attribute.Value.AsString(); value == "hunter2" || value == "secret" {
			t.Errorf("%s carries %q", attribute.Key, value)
		}
	}
}
