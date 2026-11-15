package movies

import "go.uber.org/fx"

var Module = fx.Module(
	"server/movies",
	fx.Provide(
		New,
	),
)
