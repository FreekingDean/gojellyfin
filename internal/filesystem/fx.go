package filesystem

import "go.uber.org/fx"

var Module = fx.Module(
	"filesystem",
	fx.Provide(
		New,
	),
)
