package shelves

import "go.uber.org/fx"

var Module = fx.Module(
	"shelves",
	fx.Provide(
		New,
	),
)
