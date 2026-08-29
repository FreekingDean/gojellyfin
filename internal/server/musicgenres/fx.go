package musicgenres

import "go.uber.org/fx"

var Module = fx.Module(
	"server/musicgenres",
	fx.Provide(
		New,
	),
)
