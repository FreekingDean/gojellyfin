package settings

import "go.uber.org/fx"

var Module = fx.Module(
	"server/settings",
	fx.Provide(
		New,
	),
)
