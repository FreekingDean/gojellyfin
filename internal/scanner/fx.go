package scanner

import (
	"go.uber.org/fx"
)

var Module = fx.Module(
	"scanner",
	fx.Provide(
		New,
	),
)
