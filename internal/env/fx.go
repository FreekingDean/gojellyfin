package env

import (
	"go.uber.org/fx"
)

var Module = fx.Module(
	"env",
	fx.Provide(Load),
)
