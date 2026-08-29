package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTokenFrom(t *testing.T) {
	t.Run("accepts either query spelling", func(t *testing.T) {
		tests := map[string]string{
			"/socket?ApiKey=abc":            "abc",
			"/socket?api_key=abc":           "abc",
			"/socket?apikey=abc":            "abc",
			"/socket?deviceId=x&ApiKey=abc": "abc",
			"/socket":                       "",
		}

		for target, want := range tests {
			if got := TokenFrom(httptest.NewRequest(http.MethodGet, target, nil)); got != want {
				t.Errorf("TokenFrom(%q) = %q, want %q", target, got, want)
			}
		}
	})

	t.Run("prefers the header", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/socket?ApiKey=fromquery", nil)
		r.Header.Set("Authorization", `MediaBrowser Token="fromheader", Client="test"`)

		if got := TokenFrom(r); got != "fromheader" {
			t.Errorf("TokenFrom = %q, want fromheader", got)
		}
	})

	t.Run("decodes percent encoding", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header.Set("Authorization", `MediaBrowser Client="Jellyfin%20Web", Token="a%2Bb"`)

		if got := TokenFrom(r); got != "a+b" {
			t.Errorf("TokenFrom = %q, want %q", got, "a+b")
		}
	})
}
