package instantmix

import "go.uber.org/fx"

var Module = fx.Module(
	"server/instantmix",
	fx.Provide(
		New,
	),
)
