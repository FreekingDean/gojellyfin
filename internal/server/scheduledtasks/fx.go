package scheduledtasks

import "go.uber.org/fx"

var Module = fx.Module(
	"server/scheduledtasks",
	fx.Provide(
		New,
	),
)
