package collage

import "go.uber.org/fx"

var Module = fx.Module(
	"collage",
	fx.Provide(New),
)
