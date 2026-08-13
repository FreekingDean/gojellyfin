package itemupdate

import "go.uber.org/fx"

var Module = fx.Module(
	"server/itemupdate",
	fx.Provide(
		New,
	),
)
