package mediasegment

import "go.uber.org/fx"

var Module = fx.Module(
	"server/mediasegment",
	fx.Provide(
		New,
	),
)
