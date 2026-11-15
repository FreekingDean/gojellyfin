package cards

import "go.uber.org/fx"

var Module = fx.Module(
	"cards",
	fx.Provide(New),
)
