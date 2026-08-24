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

func TestPreflightNamesTheCaller(t *testing.T) {
	recorder := preflight(t, []string{"https://gojellyfin.example.dev"}, "https://gojellyfin.example.dev")

	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "https://gojellyfin.example.dev" {
		t.Errorf("Access-Control-Allow-Origin = %q, want the caller's origin", got)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("Access-Control-Allow-Credentials = %q, want true", got)
	}
}

func TestPreflightRefusesAnOriginOutsideTheList(t *testing.T) {
	recorder := preflight(t, []string{"https://gojellyfin.example.dev"}, "https://evil.example.dev")

	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Access-Control-Allow-Origin = %q, want it absent", got)
	}
}

func TestPreflightMatchesAnOriginCaseInsensitively(t *testing.T) {
	recorder := preflight(t, []string{"HTTPS://GoJellyfin.Example.Dev"}, "https://gojellyfin.example.dev")

	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "https://gojellyfin.example.dev" {
		t.Errorf("Access-Control-Allow-Origin = %q, want the caller's origin", got)
	}
}
