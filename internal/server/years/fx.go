package years

import "go.uber.org/fx"

var Module = fx.Module(
	"server/years",
	fx.Provide(
		New,
	),
)
