package requests

import "go.uber.org/fx"

var Module = fx.Module(
	"requests",
	fx.Provide(
		New,
	),
)
