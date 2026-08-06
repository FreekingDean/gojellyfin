package server

import (
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	serverconfig "github.com/FreekingDean/gojellyfin/internal/server/config"
	serveritems "github.com/FreekingDean/gojellyfin/internal/server/items"
	serverlibraries "github.com/FreekingDean/gojellyfin/internal/server/libraries"
	serverusers "github.com/FreekingDean/gojellyfin/internal/server/users"
)

// Services sit one level shallower than the fallback, so a registered service
// wins the selector and everything else falls through to Unimplemented.
type nestedUnimplemented struct {
	api.Unimplemented
}

// Embedded field names are type names, so each service comes in under an alias
// to keep them distinct.
type (
	UsersServer     = serverusers.Server
	ItemsServer     = serveritems.Server
	LibrariesServer = serverlibraries.Server
	ConfigServer    = serverconfig.Server
)

type Server struct {
	*UsersServer
	*ItemsServer
	*LibrariesServer
	*ConfigServer

	nestedUnimplemented
}

func New(
	users *serverusers.Server,
	items *serveritems.Server,
	libraries *serverlibraries.Server,
	config *serverconfig.Server,
) *Server {
	return &Server{
		UsersServer:     users,
		ItemsServer:     items,
		LibrariesServer: libraries,
		ConfigServer:    config,
	}
}
