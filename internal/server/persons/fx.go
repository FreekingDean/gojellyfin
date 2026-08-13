package persons

import "go.uber.org/fx"

var Module = fx.Module(
	"server/persons",
	fx.Provide(
		New,
	),
)
