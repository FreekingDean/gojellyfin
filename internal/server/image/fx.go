package image

import "go.uber.org/fx"

var Module = fx.Module(
	"server/image",
	fx.Provide(
		New,
	),
)
