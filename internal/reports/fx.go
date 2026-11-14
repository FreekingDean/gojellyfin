package reports

import "go.uber.org/fx"

var Module = fx.Module(
	"reports",
	fx.Provide(
		New,
	),
)
