package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/http/mux"
)

func TestRegisterRoot(t *testing.T) {
	m := mux.New()
	registerRoot(m)
	m.HandleFunc("GET /System/Info/Public", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})

	tests := []struct {
		method   string
		path     string
		status   int
		location string
	}{
		{http.MethodGet, "/", http.StatusFound, clientPath},
		{http.MethodHead, "/", http.StatusFound, clientPath},
		{http.MethodGet, "/System/Info/Public", http.StatusTeapot, ""},
		{http.MethodPost, "/", http.StatusNotFound, ""},
	}
	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			m.ServeHTTP(recorder, httptest.NewRequest(tt.method, tt.path, nil))

			if recorder.Code != tt.status {
				t.Fatalf("status = %d, want %d", recorder.Code, tt.status)
			}
			if got := recorder.Header().Get("Location"); got != tt.location {
				t.Fatalf("Location = %q, want %q", got, tt.location)
			}
		})
	}
}
