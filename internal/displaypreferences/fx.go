package displaypreferences

import "go.uber.org/fx"

var Module = fx.Module(
	"displaypreferences",
	fx.Provide(
		New,
	),
)
