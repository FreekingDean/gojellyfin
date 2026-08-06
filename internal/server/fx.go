package server

import (
	"go.uber.org/fx"

	"github.com/FreekingDean/gojellyfin/internal/http/middleware"
	"github.com/FreekingDean/gojellyfin/internal/scanner"
	"github.com/FreekingDean/gojellyfin/internal/server/config"
	"github.com/FreekingDean/gojellyfin/internal/server/items"
	"github.com/FreekingDean/gojellyfin/internal/server/libraries"
	"github.com/FreekingDean/gojellyfin/internal/server/users"
)

var Module = fx.Module(
	"server",
	fx.Provide(
		users.New,
		items.New,
		libraries.New,
		config.New,
		sessions,
		New,
	),
	fx.Invoke(
		useScanner,
	),
)

func sessions(users *users.Server) middleware.Sessions {
	return users
}

func useScanner(libraries *libraries.Server, scanner *scanner.Scanner) {
	libraries.UseScanner(scanner)
}
