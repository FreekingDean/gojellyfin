package system

import (
	"go.uber.org/fx"
)

var Module = fx.Module(
	"system",
	fx.Provide(
		New,
	),
)
