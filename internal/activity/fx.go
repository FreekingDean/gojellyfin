package activity

import "go.uber.org/fx"

var Module = fx.Module(
	"activity",
	fx.Provide(
		New,
	),
)
