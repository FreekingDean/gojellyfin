package requests

import "go.uber.org/fx"

var Module = fx.Module(
	"server/requests",
	fx.Provide(
		New,
	),
)
