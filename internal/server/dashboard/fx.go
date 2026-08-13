package dashboard

import "go.uber.org/fx"

var Module = fx.Module(
	"server/dashboard",
	fx.Provide(
		New,
	),
)
