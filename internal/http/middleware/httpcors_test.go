package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAllowOriginDefaultsToEveryOrigin(t *testing.T) {
	allow := allowOrigin("")

	for _, origin := range []string{"https://gojellyfin.example.dev", "http://localhost:8096"} {
		if !allow(origin) {
			t.Errorf("allowOrigin(%q) = false, want true", origin)
		}
	}
}

func TestAllowOriginHonoursTheList(t *testing.T) {
	allow := allowOrigin("https://gojellyfin.example.dev/, HTTP://LOCALHOST:8096")

	for _, origin := range []string{"https://gojellyfin.example.dev", "http://localhost:8096"} {
		if !allow(origin) {
			t.Errorf("allowOrigin(%q) = false, want true", origin)
		}
	}
	if allow("https://evil.example.dev") {
		t.Error("an origin outside the list was allowed")
	}
}

// A browser rejects `Access-Control-Allow-Origin: *` on a credentialed request,
// so the header has to name the caller.
func TestPreflightReflectsTheOrigin(t *testing.T) {
	request := httptest.NewRequest(http.MethodOptions, "/Users/AuthenticateByName", nil)
	request.Header.Set("Origin", "https://gojellyfin.example.dev")
	request.Header.Set("Access-Control-Request-Method", "POST")
	request.Header.Set("Access-Control-Request-Headers", "authorization")

	recorder := httptest.NewRecorder()
	HttpCORS(http.NotFoundHandler()).ServeHTTP(recorder, request)

	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "https://gojellyfin.example.dev" {
		t.Errorf("Access-Control-Allow-Origin = %q, want the caller's origin", got)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("Access-Control-Allow-Credentials = %q, want true", got)
	}
}
