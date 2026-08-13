package environment

import "go.uber.org/fx"

var Module = fx.Module(
	"server/environment",
	fx.Provide(
		New,
	),
)
