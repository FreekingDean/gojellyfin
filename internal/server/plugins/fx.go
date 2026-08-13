package plugins

import "go.uber.org/fx"

var Module = fx.Module(
	"server/plugins",
	fx.Provide(
		New,
	),
)
