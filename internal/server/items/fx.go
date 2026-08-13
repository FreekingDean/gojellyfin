package items

import "go.uber.org/fx"

var Module = fx.Module(
	"server/items",
	fx.Provide(
		New,
	),
)
