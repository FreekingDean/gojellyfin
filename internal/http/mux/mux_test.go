package mux

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMux_ServeHTTP(t *testing.T) {
	t.Run("matches the patterns and captures their parameters", func(t *testing.T) {
		tests := []struct {
			pattern string
			path    string
			match   bool
			params  map[string]string
		}{
			{"GET /Users/{userId}", "/Users/abc", true, map[string]string{"userId": "abc"}},
			{"GET /Users/{userId}", "/users/abc", true, map[string]string{"userId": "abc"}},
			{"GET /Users/{userId}", "/Users/abc/", true, map[string]string{"userId": "abc"}},
			{"GET /Users/{userId}", "/Users/abc/def", false, nil},
			{"GET /Videos/{itemId}/stream", "/Videos/abc/stream", true, map[string]string{"itemId": "abc"}},
			{
				"GET /Videos/{itemId}/stream.{container}",
				"/Videos/abc/stream.mkv",
				true,
				map[string]string{"itemId": "abc", "container": "mkv"},
			},
			{"GET /Videos/{itemId}/stream.{container}", "/Videos/abc/stream", false, nil},
			{"GET /Branding/Css.css", "/Branding/Css.css", true, nil},
			{"GET /Branding/Css.css", "/Branding/CssXcss", false, nil},
			{"GET /web/*", "/web/assets/main.js", true, nil},
		}

		for _, test := range tests {
			m := New()
			matched := false
			m.HandleFunc(test.pattern, func(w http.ResponseWriter, r *http.Request) {
				matched = true
				for name, want := range test.params {
					if got := r.PathValue(name); got != want {
						t.Errorf("%s %s: %s = %q, want %q", test.pattern, test.path, name, got, want)
					}
				}
			})

			m.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, test.path, nil))
			if matched != test.match {
				t.Errorf("%s matched %s = %v, want %v", test.pattern, test.path, matched, test.match)
			}
		}
	})

	t.Run("the method is respected", func(t *testing.T) {
		m := New()
		matched := false
		m.HandleFunc("POST /Items", func(http.ResponseWriter, *http.Request) { matched = true })

		m.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/Items", nil))
		if matched {
			t.Error("GET matched a POST route")
		}

		m.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/Items", nil))
		if !matched {
			t.Error("POST did not match a POST route")
		}
	})
}
