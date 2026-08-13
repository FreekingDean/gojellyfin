package localization

import "go.uber.org/fx"

var Module = fx.Module(
	"server/localization",
	fx.Provide(
		New,
	),
)
