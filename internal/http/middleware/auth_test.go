package middleware

import (
	"net/http/httptest"
	"testing"
)

func TestTokenFromAcceptsEitherQuerySpelling(t *testing.T) {
	tests := map[string]string{
		"/socket?ApiKey=abc":            "abc",
		"/socket?api_key=abc":           "abc",
		"/socket?apikey=abc":            "abc",
		"/socket?deviceId=x&ApiKey=abc": "abc",
		"/socket":                       "",
	}

	for target, want := range tests {
		if got := TokenFrom(httptest.NewRequest("GET", target, nil)); got != want {
			t.Errorf("TokenFrom(%q) = %q, want %q", target, got, want)
		}
	}
}

func TestTokenFromPrefersTheHeader(t *testing.T) {
	r := httptest.NewRequest("GET", "/socket?ApiKey=fromquery", nil)
	r.Header.Set("Authorization", `MediaBrowser Token="fromheader", Client="test"`)

	if got := TokenFrom(r); got != "fromheader" {
		t.Errorf("TokenFrom = %q, want fromheader", got)
	}
}

func TestParseAuthorizationDecodesValues(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", `MediaBrowser Client="Jellyfin%20Web", Device="Chrome", Token="abc"`)

	if got := parseAuthorization(r).Client; got != "Jellyfin Web" {
		t.Errorf("Client = %q, want %q", got, "Jellyfin Web")
	}
}
