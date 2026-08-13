package filter

import "go.uber.org/fx"

var Module = fx.Module(
	"server/filter",
	fx.Provide(
		New,
	),
)
