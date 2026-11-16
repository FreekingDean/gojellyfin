package channels

import "go.uber.org/fx"

var Module = fx.Module(
	"channels",
	fx.Provide(New),
)
