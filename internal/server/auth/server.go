package auth

import (
	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/users"
)

type Server struct {
	auth  *auth.Service
	users *users.Service
}

func New(service *auth.Service, users *users.Service) *Server {
	return &Server{auth: service, users: users}
}
