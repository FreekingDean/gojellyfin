package discover

import "go.uber.org/fx"

var Module = fx.Module(
	"discover",
	fx.Provide(
		New,
	),
)
