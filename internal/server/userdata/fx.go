package userdata

import "go.uber.org/fx"

var Module = fx.Module(
	"server/userdata",
	fx.Provide(
		New,
	),
)
