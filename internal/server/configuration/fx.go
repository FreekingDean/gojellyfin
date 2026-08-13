package configuration

import "go.uber.org/fx"

var Module = fx.Module(
	"server/configuration",
	fx.Provide(
		New,
	),
)
