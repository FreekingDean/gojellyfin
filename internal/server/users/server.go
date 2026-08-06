package users

import (
	"github.com/FreekingDean/gojellyfin/internal/users"
)

type Server struct {
	users *users.Service
}

func New(service *users.Service) *Server {
	return &Server{users: service}
}
