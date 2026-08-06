package users

import (
	"github.com/FreekingDean/gojellyfin/internal/users"
)

type Server struct {
	store *users.Store
}

func New(store *users.Store) *Server {
	return &Server{store: store}
}
