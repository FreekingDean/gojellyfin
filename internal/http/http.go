package http

import (
	"context"
	"errors"
	"log"
	"net/http"

	"github.com/FreekingDean/gojellyfin/internal/http/middleware"
	"github.com/FreekingDean/gojellyfin/internal/http/mux"
	"github.com/FreekingDean/gojellyfin/internal/server"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/socket"
)

type Server struct {
	s *http.Server

	httpMiddleware []middleware.HttpMiddleware
	apiMiddleware  []api.StrictMiddlewareFunc
	apiOptions     api.StrictHTTPServerOptions
}

func New(m *mux.Mux, auth *middleware.Auth) *Server {
	return &Server{
		s: &http.Server{
			Addr: ":8081",
		},

		httpMiddleware: []middleware.HttpMiddleware{
			middleware.HttpCORS,
			middleware.HttpLogging,
		},

		apiMiddleware: []api.StrictMiddlewareFunc{
			middleware.OapiLogging,
			auth.Middleware,
		},

		apiOptions: api.StrictHTTPServerOptions{
			ResponseErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
				log.Printf("Error: %v", err)
				if errors.Is(err, api.ErrNotImplemented) {
					http.Error(w, err.Error(), http.StatusNotImplemented)
					return
				}
				if errors.Is(err, middleware.ErrUnauthorized) {
					http.Error(w, err.Error(), http.StatusUnauthorized)
					return
				}
				http.Error(w, err.Error(), http.StatusInternalServerError)
			},
		},
	}
}

func Register(s *Server, apiServer *server.Server, sock *socket.Socket, m *mux.Mux) {
	h := api.NewStrictHandlerWithOptions(apiServer, s.apiMiddleware, s.apiOptions)
	m.HandleFunc("GET /socket", sock.Handle)
	finalHandler := api.HandlerFromMux(h, m)
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
