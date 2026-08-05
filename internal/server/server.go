package server

import (
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/store"
	"github.com/FreekingDean/gojellyfin/internal/system"
)

type Server struct {
	api.Unimplemented

	id   string
	name string

	store  store.Store
	system system.Service
}

func New(store store.Store, system system.Service) *Server {
	return &Server{
		store:  store,
		system: system,
		id:     serverId,
		name:   "gojellyfin",
	}
}

func (s *Server) ID() string {
	return s.id
}

func (s *Server) Name() string {
	return s.name
}
