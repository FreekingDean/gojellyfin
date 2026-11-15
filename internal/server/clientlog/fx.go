package clientlog

import "go.uber.org/fx"

var Module = fx.Module(
	"server/clientlog",
	fx.Provide(
		New,
	),
)
