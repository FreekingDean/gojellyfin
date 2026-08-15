package http

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/env"
)

// The mux matches in registration order, so a parameter registered before a
// literal it can match swallows it: /Users/{userId}/Items/{itemId} would take
// /Users/{userId}/Items/Resume and hand "Resume" over as an item id.
func TestLegacyPatternsPutLiteralsBeforeParameters(t *testing.T) {
	patterns := legacyPatterns()

	for _, pattern := range patterns {
		for _, other := range patterns {
			if pattern == other || !shadows(other, pattern) {
				continue
			}

			if slices.Index(patterns, other) < slices.Index(patterns, pattern) {
				t.Errorf("%q is registered before %q and swallows it", other, pattern)
			}
		}
	}
}

func TestLegacyPatternsAreStable(t *testing.T) {
	first := legacyPatterns()
	for range 20 {
		if !slices.Equal(first, legacyPatterns()) {
			t.Fatal("want a stable order, got one that varies per call")
		}
	}
}

// Whether a request matching pattern would also match candidate, which is only
// true when candidate is the same shape with a parameter where pattern has a
// literal.
func shadows(candidate, pattern string) bool {
	a := strings.Split(candidate, "/")
	b := strings.Split(pattern, "/")
	if len(a) != len(b) {
		return false
	}

	loose := false
	for i := range a {
		switch {
		case a[i] == b[i]:
		case strings.HasPrefix(a[i], "{"):
			loose = true
		default:
			return false
		}
	}

	return loose
}

// Routes deliberately left without an alias, and why. Anything else in
// spec/jellyfin-hidden-routes-10.10.0.txt has to be in legacyRoutes.
var unaliased = map[string]string{
	"GET /QuickConnect/Initiate":        "the documented form is POST, so this is a method change rather than a path rewrite",
	"POST /Users/{userId}/Authenticate": "authenticating by user id has no documented replacement; AuthenticateByName takes a username",
	"POST /Users/{userId}/EasyPassword": "easy passwords were removed, with nothing to forward to",
}

func TestLegacyRoutesCoverJellyfin(t *testing.T) {
	hidden := readHiddenRoutes(t)

	for _, route := range hidden {
		_, aliased := legacyRoutes[route]
		_, excused := unaliased[route]
		if !aliased && !excused {
			t.Errorf("%q is hidden from the spec with no alias and no stated reason", route)
		}
		if aliased && excused {
			t.Errorf("%q is both aliased and listed as unaliased", route)
		}
	}

	for route := range legacyRoutes {
		if !slices.Contains(hidden, route) {
			t.Errorf("%q is aliased but is not a route Jellyfin hides", route)
		}
	}
}

func readHiddenRoutes(t *testing.T) []string {
	t.Helper()

	body, err := os.ReadFile(filepath.Join("..", "..", "spec", "jellyfin-hidden-routes-10.10.0.txt"))
	if err != nil {
		t.Fatal(err)
	}

	routes := make([]string, 0)
	for _, line := range strings.Split(string(body), "\n") {
		if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "#") {
			routes = append(routes, line)
		}
	}
	if len(routes) == 0 {
		t.Fatal("want the hidden route list, got nothing")
	}

	return routes
}

// An unconfigured server serves no cross-origin access at all, rather than
// configuring the middleware to allow every origin.
func TestCORSIsOmittedWithoutConfiguredOrigins(t *testing.T) {
	for _, test := range []struct {
		name    string
		origins []string
		want    string
	}{
		{"unset", nil, ""},
		{"configured", []string{"https://gojellyfin.example.dev"}, "https://gojellyfin.example.dev"},
	} {
		t.Run(test.name, func(t *testing.T) {
			handler := http.Handler(http.NotFoundHandler())
			for _, mw := range newHTTPMiddleware(env.Config{CORSOrigins: test.origins}) {
				handler = mw(handler)
			}

			request := httptest.NewRequest(http.MethodOptions, "/Users/AuthenticateByName", nil)
			request.Header.Set("Origin", "https://gojellyfin.example.dev")
			request.Header.Set("Access-Control-Request-Method", "POST")

			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)

			if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != test.want {
				t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, test.want)
			}
		})
	}
}
