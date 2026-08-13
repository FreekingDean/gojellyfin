package devices

import "go.uber.org/fx"

var Module = fx.Module(
	"server/devices",
	fx.Provide(
		New,
	),
)
