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
)

func TestLoggingLeavesCredentialsOut(t *testing.T) {
	var logged bytes.Buffer
	log.SetOutput(&logged)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	for _, target := range []string{
		"/QuickConnect/Connect?secret=hunter2",
		"/Videos/1/stream?api_key=hunter2",
		"/socket?ApiKey=hunter2",
	} {
		request := httptest.NewRequest(http.MethodGet, target, nil)

		handler := OapiLogging(func(ctx context.Context, w http.ResponseWriter, r *http.Request, request any) (any, error) {
			return nil, nil
		}, "Operation")
		_, _ = handler(context.Background(), httptest.NewRecorder(), request, nil)

		HttpLogging(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})).ServeHTTP(httptest.NewRecorder(), request)
	}

	if strings.Contains(logged.String(), "hunter2") {
		t.Errorf("the log carries a credential:\n%s", logged.String())
	}
	if !strings.Contains(logged.String(), "/QuickConnect/Connect?secret=redacted") {
		t.Errorf("the log lost the request it describes:\n%s", logged.String())
	}
}

func TestLoggingLetsAStreamFlush(t *testing.T) {
	var flushed error
	handler := HttpLogging(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("first fragment"))
		flushed = http.NewResponseController(w).Flush()
	}))

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/Videos/x/hls1/main/0.mp4", nil))

	if flushed != nil {
		t.Fatalf("flush = %v, want the logger to pass it through", flushed)
	}
	if !recorder.Flushed {
		t.Error("the response was never flushed")
	}
}
