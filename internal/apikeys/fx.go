package apikeys

import "go.uber.org/fx"

var Module = fx.Module(
	"apikeys",
	fx.Provide(
		New,
	),
)
