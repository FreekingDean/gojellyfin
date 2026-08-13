package mediainfo

import "go.uber.org/fx"

var Module = fx.Module(
	"server/mediainfo",
	fx.Provide(
		New,
	),
)
