package subtitle

import "go.uber.org/fx"

var Module = fx.Module(
	"server/subtitle",
	fx.Provide(
		New,
	),
)
