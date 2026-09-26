package syncplay

import "go.uber.org/fx"

var Module = fx.Module(
	"syncplay",
	fx.Provide(
		New,
	),
)
