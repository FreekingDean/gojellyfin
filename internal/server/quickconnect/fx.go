package quickconnect

import "go.uber.org/fx"

var Module = fx.Module(
	"server/quickconnect",
	fx.Provide(
		New,
	),
)
