package userlibrary

import "go.uber.org/fx"

var Module = fx.Module(
	"server/userlibrary",
	fx.Provide(
		New,
	),
)
