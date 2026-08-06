package server

import (
	"go.uber.org/fx"

	"github.com/FreekingDean/gojellyfin/internal/config"
	"github.com/FreekingDean/gojellyfin/internal/http/middleware"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
	"github.com/FreekingDean/gojellyfin/internal/scanner"
	serverconfig "github.com/FreekingDean/gojellyfin/internal/server/config"
	serveritems "github.com/FreekingDean/gojellyfin/internal/server/items"
	serverlibraries "github.com/FreekingDean/gojellyfin/internal/server/libraries"
	serverusers "github.com/FreekingDean/gojellyfin/internal/server/users"
	"github.com/FreekingDean/gojellyfin/internal/users"
)

var Module = fx.Module(
	"server",
	fx.Provide(
		users.New,
		items.New,
		libraries.New,
		config.New,

		serverusers.New,
		serveritems.New,
		serverlibraries.New,
		serverconfig.New,

		sessions,
		New,
	),
	fx.Invoke(
		useScanner,
	),
)

func sessions(users *serverusers.Server) middleware.Sessions {
	return users
}

func useScanner(libraries *serverlibraries.Server, scanner *scanner.Scanner) {
	libraries.UseScanner(scanner)
}
