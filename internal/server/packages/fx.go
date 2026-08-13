package packages

import "go.uber.org/fx"

var Module = fx.Module(
	"server/packages",
	fx.Provide(
		New,
	),
)
