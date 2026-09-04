package sources

import "go.uber.org/fx"

var Module = fx.Module(
	"server/sources",
	fx.Provide(
		New,
	),
)
