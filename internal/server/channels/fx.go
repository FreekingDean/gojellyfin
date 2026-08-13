package channels

import "go.uber.org/fx"

var Module = fx.Module(
	"server/channels",
	fx.Provide(
		New,
	),
)
