package http

import (
	"context"
	"errors"
	"log"
	"net/http"

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
	// Parameter binding failures answer 400 without reaching a handler, so this
	// is the only place the reason is visible.
	finalHandler := api.HandlerWithOptions(h, api.StdHTTPServerOptions{
		BaseRouter: m,
		ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("bad request: %s %s: %v", r.Method, r.RequestURI, err)
			http.Error(w, err.Error(), http.StatusBadRequest)
		},
	})
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
	return
}
