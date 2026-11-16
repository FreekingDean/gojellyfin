package middleware

import (
	"bytes"
	"context"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/observability/tracing"
)

func TestLoggingNamesTheTrace(t *testing.T) {
	var logged bytes.Buffer
	log.SetOutput(&logged)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	traces, _ := tracing.Recorded()
	ctx, span := traces.StartRequest(context.Background(), http.Header{}, "GetItems", http.MethodGet)
	defer span.End()

	handler := OapiLogging(func(ctx context.Context, w http.ResponseWriter, r *http.Request, request any) (any, error) {
		return nil, nil
	}, "GetItems")
	_, _ = handler(ctx, httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/Items", nil), nil)

	if want := "trace=" + tracing.TraceID(ctx); !strings.Contains(logged.String(), want) {
		t.Errorf("the request log does not name its trace %q:\n%s", want, logged.String())
	}
}
