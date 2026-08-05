package server

import (
	"gorm.io/gorm"

	"github.com/FreekingDean/gojellyfin/internal/scanner"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/users"
	"github.com/FreekingDean/gojellyfin/internal/store"
	"github.com/FreekingDean/gojellyfin/internal/system"
)

const (
	serverId     = "e10a32fca79342d7b8b9d96e255ce1bc"
	rootFolderId = "e9d5075a555c1cbc394eec4cef295274"
)

// Services sit one level shallower than the fallback, so a registered service
// wins the selector and everything else falls through to Unimplemented.
type nestedUnimplemented struct {
	api.Unimplemented
}

type UsersServer = users.Server

type Server struct {
	*UsersServer

	id   string
	name string

	store   store.Store
	system  system.Service
	scanner *scanner.Scanner

	nestedUnimplemented
}

func New(db *gorm.DB, store store.Store, system system.Service, scanner *scanner.Scanner) *Server {
	return &Server{
		UsersServer: users.New(db),
		store:       store,
		system:      system,
		scanner:     scanner,
		id:          serverId,
		name:        "gojellyfin",
	}
}

func (s *Server) ID() string {
	return s.id
}

func (s *Server) Name() string {
	return s.name
}
