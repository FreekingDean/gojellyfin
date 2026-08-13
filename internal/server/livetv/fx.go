package livetv

import "go.uber.org/fx"

var Module = fx.Module(
	"server/livetv",
	fx.Provide(
		New,
	),
)
