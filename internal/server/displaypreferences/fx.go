package displaypreferences

import "go.uber.org/fx"

var Module = fx.Module(
	"server/displaypreferences",
	fx.Provide(
		New,
	),
)
