package library

import "go.uber.org/fx"

var Module = fx.Module(
	"server/library",
	fx.Provide(
		New,
	),
)
