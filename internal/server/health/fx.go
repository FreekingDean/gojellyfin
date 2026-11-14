package health

import "go.uber.org/fx"

var Module = fx.Module(
	"server/health",
	fx.Provide(
		New,
	),
)
