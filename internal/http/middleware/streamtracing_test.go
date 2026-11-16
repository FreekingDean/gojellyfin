package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/observability/tracing"
)

func TestStreamTracing(t *testing.T) {
	t.Run("the span ends at the first byte, before the relay", func(t *testing.T) {
		traces, recorder := tracing.Recorded()
		var during []string
		var traced string
		handler := StreamTracing(traces, "GetVideoStream", func(w http.ResponseWriter, r *http.Request) {
			traced = tracing.TraceID(r.Context())
			_, _ = w.Write([]byte("first"))
			during = recorder.Names()
			_, _ = w.Write([]byte("rest"))
		})

		handler(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/Videos/6f1b/stream?api_key=hunter2", nil))

		if len(during) != 1 || during[0] != "GetVideoStream" {
			t.Errorf("after the first byte want the span ended, got %v", during)
		}
		if traced == "" {
			t.Error("the handler's context carries no span for ffmpeg to hang off")
		}
		if values := recorder.Values(); values["http.response.status_code"] != "200" {
			t.Errorf("status = %q", values["http.response.status_code"])
		}
	})

	t.Run("a refusal ends it with the status", func(t *testing.T) {
		traces, recorder := tracing.Recorded()
		handler := StreamTracing(traces, "GetVideoStream", func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "busy", http.StatusServiceUnavailable)
		})

		handler(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/Videos/6f1b/stream", nil))

		if values := recorder.Values(); values["http.response.status_code"] != "503" {
			t.Errorf("status = %q", values["http.response.status_code"])
		}
	})

	t.Run("a handler that writes nothing still ends it", func(t *testing.T) {
		traces, recorder := tracing.Recorded()
		StreamTracing(traces, "HeadVideoStream", func(w http.ResponseWriter, r *http.Request) {})(
			httptest.NewRecorder(), httptest.NewRequest(http.MethodHead, "/Videos/6f1b/stream", nil))

		if names := recorder.Names(); len(names) != 1 {
			t.Errorf("want one ended span, got %v", names)
		}
	})

	t.Run("the body still reaches the client whole", func(t *testing.T) {
		traces, _ := tracing.Recorded()
		response := httptest.NewRecorder()
		StreamTracing(traces, "GetVideoStream", func(w http.ResponseWriter, r *http.Request) {
			http.ServeContent(w, r, "a.mp4", time.Time{}, strings.NewReader("whole file"))
		})(response, httptest.NewRequest(http.MethodGet, "/Videos/6f1b/stream", nil))

		if response.Body.String() != "whole file" {
			t.Errorf("body = %q", response.Body.String())
		}
	})

	t.Run("with tracing off the handler is untouched", func(t *testing.T) {
		traces, err := tracing.New(env.Config{})
		if err != nil {
			t.Fatal(err)
		}
		called := false
		StreamTracing(traces, "GetVideoStream", func(w http.ResponseWriter, r *http.Request) {
			_, called = w.(*httptest.ResponseRecorder)
		})(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/Videos/6f1b/stream", nil))

		if !called {
			t.Error("the writer was wrapped with tracing off")
		}
	})
}
