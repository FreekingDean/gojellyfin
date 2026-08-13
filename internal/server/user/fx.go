package user

import "go.uber.org/fx"

var Module = fx.Module(
	"server/user",
	fx.Provide(
		New,
	),
)
