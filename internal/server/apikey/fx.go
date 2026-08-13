package apikey

import "go.uber.org/fx"

var Module = fx.Module(
	"server/apikey",
	fx.Provide(
		New,
	),
)
