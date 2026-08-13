package suggestions

import "go.uber.org/fx"

var Module = fx.Module(
	"server/suggestions",
	fx.Provide(
		New,
	),
)
