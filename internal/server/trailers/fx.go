package trailers

import "go.uber.org/fx"

var Module = fx.Module(
	"server/trailers",
	fx.Provide(
		New,
	),
)
