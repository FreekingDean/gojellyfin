package userviews

import "go.uber.org/fx"

var Module = fx.Module(
	"server/userviews",
	fx.Provide(
		New,
	),
)
