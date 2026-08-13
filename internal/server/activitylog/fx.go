package activitylog

import "go.uber.org/fx"

var Module = fx.Module(
	"server/activitylog",
	fx.Provide(
		New,
	),
)
