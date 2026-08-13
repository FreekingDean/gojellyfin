package playstate

import "go.uber.org/fx"

var Module = fx.Module(
	"server/playstate",
	fx.Provide(
		New,
	),
)
