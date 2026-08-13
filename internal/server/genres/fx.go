package genres

import "go.uber.org/fx"

var Module = fx.Module(
	"server/genres",
	fx.Provide(
		New,
	),
)
