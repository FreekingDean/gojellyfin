package reports

import "go.uber.org/fx"

var Module = fx.Module(
	"server/reports",
	fx.Provide(
		New,
	),
)
