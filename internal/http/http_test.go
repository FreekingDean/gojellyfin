package http

import (
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/http/mux"
)

func TestLegacyPatterns(t *testing.T) {
	t.Run("registers literals before the parameters they shadow", func(t *testing.T) {
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
	})

	t.Run("returns the same order on every call", func(t *testing.T) {
		first := legacyPatterns()
		for range 20 {
			if !slices.Equal(first, legacyPatterns()) {
				t.Fatal("want a stable order, got one that varies per call")
			}
		}
	})
}

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

const notHLS = "HLS is not served: a transcode is one progressive response, so there is no playlist or segment to answer"

var unaliased = map[string]string{
	"POST /Users/{userId}/Authenticate": "authenticating by user id has no documented replacement; AuthenticateByName takes a username",
	"GET /Artists/InstantMix":           "the artist id rides in the query rather than the path, and there is no music for an instant mix to draw on",
	"GET /LiveTv/Recordings/Groups":     "live TV has no recordings (#527)",
	"GET /LiveTv/Recordings/Series":     "live TV has no recordings (#527)",
	"DELETE /Videos/ActiveEncodings":    "served beside the stream routes by stream.Handler.StopEncodings, because jellyfin-web refuses to seek until it succeeds",

	"GET /Audio/{itemId}/hls/{segmentId}/stream.aac":                       notHLS,
	"GET /Audio/{itemId}/hls/{segmentId}/stream.mp3":                       notHLS,
	"GET /Audio/{itemId}/hls1/{playlistId}/{segmentId}.{container}":        notHLS,
	"GET /Audio/{itemId}/main.m3u8":                                        notHLS,
	"GET /Audio/{itemId}/master.m3u8":                                      notHLS,
	"HEAD /Audio/{itemId}/master.m3u8":                                     notHLS,
	"GET /Videos/{itemId}/hls/{playlistId}/stream.m3u8":                    notHLS,
	"GET /Videos/{itemId}/hls/{playlistId}/{segmentId}.{segmentContainer}": notHLS,
	"GET /Videos/{itemId}/hls1/{playlistId}/{segmentId}.{container}":       notHLS,
	"GET /Videos/{itemId}/live.m3u8":                                       notHLS,
	"GET /Videos/{itemId}/main.m3u8":                                       notHLS,
	"GET /Videos/{itemId}/master.m3u8":                                     notHLS,
	"HEAD /Videos/{itemId}/master.m3u8":                                    notHLS,
}

func TestLegacyRoutes(t *testing.T) {
	hidden := readHiddenRoutes(t)

	for _, route := range hidden {
		_, aliased := legacyRoutes[route]
		_, converted := legacyPlaybackRoutes[route]
		aliased = aliased || converted
		_, excused := unaliased[route]
		if !aliased && !excused {
			t.Errorf("%q is hidden from the spec with no alias and no stated reason", route)
		}
		if aliased && excused {
			t.Errorf("%q is both aliased and listed as unaliased", route)
		}
	}

	for _, aliases := range []map[string]string{legacyRoutes, legacyPlaybackRoutes, unaliased} {
		for route := range aliases {
			if !slices.Contains(hidden, route) {
				t.Errorf("%q is listed but is not a route Jellyfin hides", route)
			}
		}
	}
}

func readHiddenRoutes(t *testing.T) []string {
	t.Helper()

	body, err := os.ReadFile(filepath.Join("..", "..", "spec", "jellyfin-hidden-routes-12.0.0.txt"))
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

func TestNewHTTPMiddleware(t *testing.T) {
	for _, test := range []struct {
		name    string
		origins []string
		origin  string
		want    string
	}{
		{"unset", nil, "https://gojellyfin.example.dev", ""},
		{"configured", []string{"https://gojellyfin.example.dev"}, "https://gojellyfin.example.dev", "https://gojellyfin.example.dev"},
		{"cast receiver unset", nil, "https://apps.jellyfin.org", "https://apps.jellyfin.org"},
		{"cast receiver configured", []string{"https://gojellyfin.example.dev"}, "https://apps.jellyfin.org", "https://apps.jellyfin.org"},
	} {
		t.Run(test.name, func(t *testing.T) {
			handler := http.Handler(http.NotFoundHandler())
			for _, mw := range newHTTPMiddleware(env.Config{CORSOrigins: test.origins}) {
				handler = mw(handler)
			}

			request := httptest.NewRequest(http.MethodOptions, "/Users/AuthenticateByName", nil)
			request.Header.Set("Origin", test.origin)
			request.Header.Set("Access-Control-Request-Method", "POST")

			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)

			if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != test.want {
				t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, test.want)
			}
		})
	}
}

func TestLegacyPlaybackRoute(t *testing.T) {
	m := mux.New()
	var method, path, apiKey string
	var report map[string]any
	for _, target := range legacyPlaybackRoutes {
		m.HandleFunc("POST "+target, func(w http.ResponseWriter, r *http.Request) {
			method, path, apiKey = r.Method, r.URL.Path, r.URL.Query().Get("api_key")
			report = map[string]any{}
			if err := json.NewDecoder(r.Body).Decode(&report); err != nil {
				t.Errorf("the body is not json: %v", err)
			}
			w.WriteHeader(http.StatusNoContent)
		})
	}
	for pattern, target := range legacyPlaybackRoutes {
		m.HandleFunc(pattern, legacyPlaybackRoute(m, target))
	}

	t.Run("reports progress as the documented body", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		m.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/PlayingItems/abc/Progress?positionTicks=420&isPaused=true&mediaSourceId=src&api_key=token&unknown=1", nil))

		if recorder.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 204", recorder.Code)
		}
		if method != http.MethodPost || path != "/Sessions/Playing/Progress" {
			t.Errorf("forwarded to %s %s, want POST /Sessions/Playing/Progress", method, path)
		}
		if apiKey != "token" {
			t.Errorf("api_key = %q, want the token kept for authentication", apiKey)
		}
		want := map[string]any{"ItemId": "abc", "PositionTicks": float64(420), "IsPaused": true, "MediaSourceId": "src"}
		if !maps.Equal(report, want) {
			t.Errorf("report = %v, want %v", report, want)
		}
	})

	t.Run("reports a stop", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		m.ServeHTTP(recorder, httptest.NewRequest(http.MethodDelete, "/PlayingItems/abc?positionTicks=9", nil))

		if path != "/Sessions/Playing/Stopped" || report["PositionTicks"] != float64(9) {
			t.Errorf("forwarded %v to %s, want the position at /Sessions/Playing/Stopped", report, path)
		}
	})

	t.Run("refuses a position that is not a number", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		m.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/PlayingItems/abc/Progress?positionTicks=soon", nil))

		if recorder.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", recorder.Code)
		}
	})
}
