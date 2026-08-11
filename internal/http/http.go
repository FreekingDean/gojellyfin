package http

import (
	"context"
	"errors"
	"log"
	"net/http"
	"regexp"
	"slices"
	"strings"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/http/middleware"
	"github.com/FreekingDean/gojellyfin/internal/http/mux"
	"github.com/FreekingDean/gojellyfin/internal/server"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/socket"
	"github.com/FreekingDean/gojellyfin/internal/server/stream"
)

var streamRoutes = []string{
	"GET /Videos/{itemId}/stream",
	"HEAD /Videos/{itemId}/stream",
	"GET /Videos/{itemId}/stream.{container}",
	"HEAD /Videos/{itemId}/stream.{container}",
	"GET /Audio/{itemId}/stream",
	"HEAD /Audio/{itemId}/stream",
	"GET /Audio/{itemId}/stream.{container}",
	"HEAD /Audio/{itemId}/stream.{container}",
}

var universalAudioRoutes = []string{
	"GET /Audio/{itemId}/universal",
	"HEAD /Audio/{itemId}/universal",
}

// Jellyfin serves these but hides them from its own OpenAPI document with
// [ApiExplorerSettings(IgnoreApi = true)], so the generated routes cannot
// carry them while clients still ask for them. Each maps to the documented
// spelling, which takes the user in the query string instead of the path.
var legacyRoutes = map[string]string{
	"GET /Users/{userId}/Items":                           "/Items",
	"GET /Users/{userId}/Items/Latest":                    "/Items/Latest",
	"GET /Users/{userId}/Items/Resume":                    "/UserItems/Resume",
	"GET /Users/{userId}/Items/Root":                      "/Items/Root",
	"GET /Users/{userId}/Items/{itemId}":                  "/Items/{itemId}",
	"GET /Users/{userId}/Items/{itemId}/Intros":           "/Items/{itemId}/Intros",
	"GET /Users/{userId}/Items/{itemId}/LocalTrailers":    "/Items/{itemId}/LocalTrailers",
	"GET /Users/{userId}/Items/{itemId}/SpecialFeatures":  "/Items/{itemId}/SpecialFeatures",
	"GET /Users/{userId}/Items/{itemId}/UserData":         "/UserItems/{itemId}/UserData",
	"POST /Users/{userId}/Items/{itemId}/UserData":        "/UserItems/{itemId}/UserData",
	"POST /Users/{userId}/Items/{itemId}/Rating":          "/UserItems/{itemId}/Rating",
	"DELETE /Users/{userId}/Items/{itemId}/Rating":        "/UserItems/{itemId}/Rating",
	"POST /Users/{userId}/FavoriteItems/{itemId}":         "/UserFavoriteItems/{itemId}",
	"DELETE /Users/{userId}/FavoriteItems/{itemId}":       "/UserFavoriteItems/{itemId}",
	"POST /Users/{userId}/PlayedItems/{itemId}":           "/UserPlayedItems/{itemId}",
	"DELETE /Users/{userId}/PlayedItems/{itemId}":         "/UserPlayedItems/{itemId}",
	"POST /Users/{userId}/PlayingItems/{itemId}":          "/PlayingItems/{itemId}",
	"DELETE /Users/{userId}/PlayingItems/{itemId}":        "/PlayingItems/{itemId}",
	"POST /Users/{userId}/PlayingItems/{itemId}/Progress": "/PlayingItems/{itemId}/Progress",
	"GET /Users/{userId}/Views":                           "/UserViews",
	"GET /Users/{userId}/GroupingOptions":                 "/UserViews/GroupingOptions",
	"GET /Users/{userId}/Suggestions":                     "/Items/Suggestions",
	"GET /Users/{userId}/Images/{imageType}":              "/UserImage",
	"HEAD /Users/{userId}/Images/{imageType}":             "/UserImage",
	"POST /Users/{userId}/Images/{imageType}":             "/UserImage",
	"DELETE /Users/{userId}/Images/{imageType}":           "/UserImage",
	// Jellyfin drops the type and index itself: a user has one image, and its
	// own legacy handlers document both parameters as unused.
	"GET /Users/{userId}/Images/{imageType}/{imageIndex}":  "/UserImage",
	"HEAD /Users/{userId}/Images/{imageType}/{imageIndex}": "/UserImage",
	"POST /Users/{userId}/Images/{imageType}/{index}":      "/UserImage",
	"DELETE /Users/{userId}/Images/{imageType}/{index}":    "/UserImage",
	"POST /Users/{userId}/Configuration":                   "/Users/Configuration",
	"POST /Users/{userId}/Password":                        "/Users/Password",
	"POST /Users/{userId}":                                 "/Users",
}

var routeParam = regexp.MustCompile(`\{([^}]+)\}`)

// The mux matches in registration order and has no notion of specificity, so
// a literal has to be registered before a parameter that would swallow it —
// /Users/{userId}/Items/Resume before /Users/{userId}/Items/{itemId}. Map
// order is random, so leaving this to chance breaks a different route on
// every boot.
func legacyPatterns() []string {
	patterns := make([]string, 0, len(legacyRoutes))
	for pattern := range legacyRoutes {
		patterns = append(patterns, pattern)
	}

	slices.SortFunc(patterns, func(a, b string) int {
		if byLiteral := literalSegments(b) - literalSegments(a); byLiteral != 0 {
			return byLiteral
		}

		return strings.Compare(a, b)
	})

	return patterns
}

func literalSegments(pattern string) int {
	count := 0
	for _, segment := range strings.Split(pattern, "/") {
		if !strings.Contains(segment, "{") {
			count++
		}
	}

	return count
}

func legacyRoute(m *mux.Mux, target string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		query.Set("userId", r.PathValue("userId"))

		r.URL.Path = routeParam.ReplaceAllStringFunc(target, func(param string) string {
			return r.PathValue(strings.Trim(param, "{}"))
		})
		r.URL.RawQuery = query.Encode()

		m.ServeHTTP(w, r)
	}
}

type Server struct {
	s *http.Server

	httpMiddleware []middleware.HttpMiddleware
	apiMiddleware  []api.StrictMiddlewareFunc
	apiOptions     api.StrictHTTPServerOptions
}

func New(m *mux.Mux, authMiddleware *middleware.Auth) *Server {
	return &Server{
		s: &http.Server{
			Addr: ":8081",
		},

		httpMiddleware: []middleware.HttpMiddleware{
			middleware.HttpCORS,
			middleware.HttpLogging,
			middleware.HttpCanonicalQuery,
		},

		apiMiddleware: []api.StrictMiddlewareFunc{
			middleware.OapiLogging,
			authMiddleware.Middleware,
		},

		apiOptions: api.StrictHTTPServerOptions{
			// Body decoding failures answer 400 from inside the generated
			// wrapper; without this the reason is thrown away.
			RequestErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
				log.Printf("bad request: %s %s: %v", r.Method, r.RequestURI, err)
				http.Error(w, err.Error(), http.StatusBadRequest)
			},
			ResponseErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
				log.Printf("Error: %v", err)
				if errors.Is(err, api.ErrNotImplemented) {
					http.Error(w, err.Error(), http.StatusNotImplemented)
					return
				}
				if errors.Is(err, auth.ErrUnauthorized) {
					http.Error(w, err.Error(), http.StatusUnauthorized)
					return
				}
				http.Error(w, err.Error(), http.StatusInternalServerError)
			},
		},
	}
}

func Register(s *Server, apiServer *server.Server, sock *socket.Socket, streams *stream.Handler, m *mux.Mux) {
	h := api.NewStrictHandlerWithOptions(apiServer, s.apiMiddleware, s.apiOptions)
	m.HandleFunc("GET /socket", sock.Handle)

	for _, pattern := range streamRoutes {
		m.HandleFunc(pattern, streams.Serve)
	}
	for _, pattern := range universalAudioRoutes {
		m.HandleFunc(pattern, streams.ServeUniversal)
	}
	// Parameter binding failures answer 400 without reaching a handler, so this
	// is the only place the reason is visible.
	finalHandler := api.HandlerWithOptions(h, api.StdHTTPServerOptions{
		BaseRouter: m,
		ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("bad request: %s %s: %v", r.Method, r.RequestURI, err)
			http.Error(w, err.Error(), http.StatusBadRequest)
		},
	})
	// After the generated routes: the mux matches in registration order, so a
	// documented literal wins any overlap with a legacy pattern, and a rewrite
	// cannot re-enter the pattern it came from.
	for _, pattern := range legacyPatterns() {
		m.HandleFunc(pattern, legacyRoute(m, legacyRoutes[pattern]))
	}
	for _, mw := range s.httpMiddleware {
		finalHandler = mw(finalHandler)
	}
	s.s.Handler = finalHandler
}

func (s *Server) ListenAndServe() {
	if err := s.s.ListenAndServe(); err != nil {
		if err != http.ErrServerClosed {
			log.Printf("ListenAndServe error: %v", err)
		}
	}
}

func (s *Server) Shutdown(ctx context.Context) {
	if err := s.s.Shutdown(ctx); err != nil {
		log.Printf("Shutdown error: %v", err)
	}
}
