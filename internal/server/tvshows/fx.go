package tvshows

import "go.uber.org/fx"

var Module = fx.Module(
	"server/tvshows",
	fx.Provide(
		New,
	),
)
