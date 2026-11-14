package backup

import "go.uber.org/fx"

var Module = fx.Module(
	"server/backup",
	fx.Provide(
		New,
	),
)
