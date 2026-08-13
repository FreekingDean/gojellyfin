package items

import "go.uber.org/fx"

var Module = fx.Module(
	"items",
	fx.Provide(
		New,
	),
)
