package branding

import "go.uber.org/fx"

var Module = fx.Module(
	"server/branding",
	fx.Provide(
		New,
	),
)
