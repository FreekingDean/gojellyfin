package mux

import "go.uber.org/fx"

var Module = fx.Module(
	"mux",
	fx.Provide(
		New,
	),
)
