package quickconnect

import "go.uber.org/fx"

var Module = fx.Module(
	"quickconnect",
	fx.Provide(
		New,
	),
)
