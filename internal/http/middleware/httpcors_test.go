package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func preflight(t *testing.T, origins []string, origin string) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequest(http.MethodOptions, "/Users/AuthenticateByName", nil)
	request.Header.Set("Origin", origin)
	request.Header.Set("Access-Control-Request-Method", "POST")
	request.Header.Set("Access-Control-Request-Headers", "authorization")

	recorder := httptest.NewRecorder()
	HttpCORS(origins)(http.NotFoundHandler()).ServeHTTP(recorder, request)

	return recorder
}

func TestPreflight(t *testing.T) {
	tests := []struct {
		name            string
		origins         []string
		origin          string
		wantOrigin      string
		wantCredentials string
	}{
		{
			name:            "names the caller",
			origins:         []string{"https://gojellyfin.example.dev"},
			origin:          "https://gojellyfin.example.dev",
			wantOrigin:      "https://gojellyfin.example.dev",
			wantCredentials: "true",
		},
		{
			name:    "refuses an origin outside the list",
			origins: []string{"https://gojellyfin.example.dev"},
			origin:  "https://evil.example.dev",
		},
		{
			name:            "matches an origin case insensitively",
			origins:         []string{"HTTPS://GoJellyfin.Example.Dev"},
			origin:          "https://gojellyfin.example.dev",
			wantOrigin:      "https://gojellyfin.example.dev",
			wantCredentials: "true",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := preflight(t, test.origins, test.origin)

			if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != test.wantOrigin {
				t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, test.wantOrigin)
			}
			if got := recorder.Header().Get("Access-Control-Allow-Credentials"); got != test.wantCredentials {
				t.Errorf("Access-Control-Allow-Credentials = %q, want %q", got, test.wantCredentials)
			}
		})
	}
}
