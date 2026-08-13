package syncplay

import "go.uber.org/fx"

var Module = fx.Module(
	"server/syncplay",
	fx.Provide(
		New,
	),
)
