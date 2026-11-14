package artists

import "go.uber.org/fx"

var Module = fx.Module(
	"server/artists",
	fx.Provide(
		New,
	),
)
