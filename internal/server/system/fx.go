package system

import "go.uber.org/fx"

var Module = fx.Module(
	"server/system",
	fx.Provide(
		New,
	),
)
