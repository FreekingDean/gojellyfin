package server

import (
	"github.com/FreekingDean/gojellyfin/internal/config"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/users"
)

// Services sit one level shallower than the fallback, so a registered service
// wins the selector and everything else falls through to Unimplemented.
type nestedUnimplemented struct {
	api.Unimplemented
}

// Embedded field names are type names, so each service comes in under an alias
// to keep them distinct.
type (
	UsersServer     = users.Server
	ItemsServer     = items.Server
	LibrariesServer = libraries.Server
	ConfigServer    = config.Server
)

type Server struct {
	*UsersServer
	*ItemsServer
	*LibrariesServer
	*ConfigServer

	nestedUnimplemented
}

func New(
	users *users.Server,
	items *items.Server,
	libraries *libraries.Server,
	config *config.Server,
) *Server {
	return &Server{
		UsersServer:     users,
		ItemsServer:     items,
		LibrariesServer: libraries,
		ConfigServer:    config,
	}
}
