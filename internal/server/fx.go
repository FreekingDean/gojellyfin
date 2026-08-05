package server

import (
	"go.uber.org/fx"

	"github.com/FreekingDean/gojellyfin/internal/http/middleware"
	"github.com/FreekingDean/gojellyfin/internal/server/users"
)

var Module = fx.Module(
	"server",
	fx.Provide(
		users.New,
		sessions,
		New,
	),
)

func sessions(users *users.Server) middleware.Sessions {
	return users
}
