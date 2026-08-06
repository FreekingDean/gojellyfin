package server

import (
	"go.uber.org/fx"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/config"
	"github.com/FreekingDean/gojellyfin/internal/http/middleware"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
	"github.com/FreekingDean/gojellyfin/internal/scanner"
	serverauth "github.com/FreekingDean/gojellyfin/internal/server/auth"
	serverconfig "github.com/FreekingDean/gojellyfin/internal/server/config"
	serveritems "github.com/FreekingDean/gojellyfin/internal/server/items"
	serverlibraries "github.com/FreekingDean/gojellyfin/internal/server/libraries"
	serverusers "github.com/FreekingDean/gojellyfin/internal/server/users"
	"github.com/FreekingDean/gojellyfin/internal/users"
)

var Module = fx.Module(
	"server",
	fx.Provide(
		auth.New,
		users.New,
		items.New,
		libraries.New,
		config.New,

		serverauth.New,
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

func sessions(auth *serverauth.Server) middleware.Sessions {
	return auth
}

func useScanner(libraries *serverlibraries.Server, scanner *scanner.Scanner) {
	libraries.UseScanner(scanner)
}
