package sources

import "go.uber.org/fx"

var Module = fx.Module(
	"sources",
	fx.Provide(
		New,
	),
)
