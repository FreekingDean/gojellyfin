package http

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"slices"
	"strings"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/http/middleware"
	"github.com/FreekingDean/gojellyfin/internal/http/mux"
	"github.com/FreekingDean/gojellyfin/internal/observability/tracing"
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

var legacyRoutes = map[string]string{
	"GET /Users/{userId}/Items":                            "/Items",
	"GET /Users/{userId}/Items/Latest":                     "/Items/Latest",
	"GET /Users/{userId}/Items/Resume":                     "/UserItems/Resume",
	"GET /Users/{userId}/Items/Root":                       "/Items/Root",
	"GET /Users/{userId}/Items/{itemId}":                   "/Items/{itemId}",
	"GET /Users/{userId}/Items/{itemId}/Intros":            "/Items/{itemId}/Intros",
	"GET /Users/{userId}/Items/{itemId}/LocalTrailers":     "/Items/{itemId}/LocalTrailers",
	"GET /Users/{userId}/Items/{itemId}/SpecialFeatures":   "/Items/{itemId}/SpecialFeatures",
	"GET /Users/{userId}/Items/{itemId}/UserData":          "/UserItems/{itemId}/UserData",
	"POST /Users/{userId}/Items/{itemId}/UserData":         "/UserItems/{itemId}/UserData",
	"POST /Users/{userId}/Items/{itemId}/Rating":           "/UserItems/{itemId}/Rating",
	"DELETE /Users/{userId}/Items/{itemId}/Rating":         "/UserItems/{itemId}/Rating",
	"POST /Users/{userId}/FavoriteItems/{itemId}":          "/UserFavoriteItems/{itemId}",
	"DELETE /Users/{userId}/FavoriteItems/{itemId}":        "/UserFavoriteItems/{itemId}",
	"POST /Users/{userId}/PlayedItems/{itemId}":            "/UserPlayedItems/{itemId}",
	"DELETE /Users/{userId}/PlayedItems/{itemId}":          "/UserPlayedItems/{itemId}",
	"POST /Users/{userId}/PlayingItems/{itemId}":           "/PlayingItems/{itemId}",
	"DELETE /Users/{userId}/PlayingItems/{itemId}":         "/PlayingItems/{itemId}",
	"POST /Users/{userId}/PlayingItems/{itemId}/Progress":  "/PlayingItems/{itemId}/Progress",
	"GET /Users/{userId}/Views":                            "/UserViews",
	"GET /Users/{userId}/GroupingOptions":                  "/UserViews/GroupingOptions",
	"GET /Users/{userId}/Suggestions":                      "/Items/Suggestions",
	"GET /Users/{userId}/Images/{imageType}":               "/UserImage",
	"HEAD /Users/{userId}/Images/{imageType}":              "/UserImage",
	"POST /Users/{userId}/Images/{imageType}":              "/UserImage",
	"DELETE /Users/{userId}/Images/{imageType}":            "/UserImage",
	"GET /Users/{userId}/Images/{imageType}/{imageIndex}":  "/UserImage",
	"HEAD /Users/{userId}/Images/{imageType}/{imageIndex}": "/UserImage",
	"POST /Users/{userId}/Images/{imageType}/{index}":      "/UserImage",
	"DELETE /Users/{userId}/Images/{imageType}/{index}":    "/UserImage",
	"POST /Users/{userId}/Configuration":                   "/Users/Configuration",
	"POST /Users/{userId}/Password":                        "/Users/Password",
	"POST /Users/{userId}":                                 "/Users",
}

var routeParam = regexp.MustCompile(`\{([^}]+)\}`)

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

func newHTTPMiddleware(config env.Config) []middleware.HttpMiddleware {
	stack := make([]middleware.HttpMiddleware, 0, 3)
	if len(config.CORSOrigins) > 0 {
		stack = append(stack, middleware.HttpCORS(config.CORSOrigins))
	}

	return append(stack, middleware.HttpLogging, middleware.HttpCanonicalQuery)
}

func newAPIMiddleware(tracing *tracing.Tracing, tracingMiddleware *middleware.OapiTracing, authMiddleware *middleware.Auth, policies middleware.Policies) []api.StrictMiddlewareFunc {
	stack := []api.StrictMiddlewareFunc{
		middleware.OapiLogging,
		middleware.Authorize(policies),
		authMiddleware.Middleware,
	}
	if tracing.Enabled() {
		stack = append(stack, tracingMiddleware.Middleware)
	}

	return stack
}

func New(config env.Config, m *mux.Mux, authMiddleware *middleware.Auth, tracing *tracing.Tracing, tracingMiddleware *middleware.OapiTracing, policies middleware.Policies) *Server {
	return &Server{
		s: &http.Server{
			Addr: fmt.Sprintf(":%d", config.HTTPPort),
		},

		httpMiddleware: newHTTPMiddleware(config),

		apiMiddleware: newAPIMiddleware(tracing, tracingMiddleware, authMiddleware, policies),

		apiOptions: api.StrictHTTPServerOptions{
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
				if errors.Is(err, auth.ErrForbidden) {
					http.Error(w, err.Error(), http.StatusForbidden)
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
	finalHandler := api.HandlerWithOptions(h, api.StdHTTPServerOptions{
		BaseRouter: m,
		ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("bad request: %s %s: %v", r.Method, r.RequestURI, err)
			http.Error(w, err.Error(), http.StatusBadRequest)
		},
	})
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
