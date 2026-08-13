package timesync

import "go.uber.org/fx"

var Module = fx.Module(
	"server/timesync",
	fx.Provide(
		New,
	),
)
