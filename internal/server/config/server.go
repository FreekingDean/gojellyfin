package config

import (
	"github.com/FreekingDean/gojellyfin/internal/config"
	"github.com/FreekingDean/gojellyfin/internal/system"
)

type Server struct {
	config *config.Service
	system system.Service
}

func New(service *config.Service, system system.Service) *Server {
	return &Server{config: service, system: system}
}
