package libraries

import "go.uber.org/fx"

var Module = fx.Module(
	"libraries",
	fx.Provide(
		New,
	),
)
