package studios

import "go.uber.org/fx"

var Module = fx.Module(
	"server/studios",
	fx.Provide(
		New,
	),
)
