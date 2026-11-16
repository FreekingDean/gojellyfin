package videos

import "go.uber.org/fx"

var Module = fx.Module(
	"server/videos",
	fx.Provide(
		New,
	),
)
