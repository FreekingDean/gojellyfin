package session

import "go.uber.org/fx"

var Module = fx.Module(
	"server/session",
	fx.Provide(
		New,
	),
)
