package config

import (
	"github.com/FreekingDean/gojellyfin/internal/config"
	"github.com/FreekingDean/gojellyfin/internal/system"
)

type Server struct {
	store  *config.Store
	system system.Service
}

func New(store *config.Store, system system.Service) *Server {
	return &Server{store: store, system: system}
}
