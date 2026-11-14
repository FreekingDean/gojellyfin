package sso

import "go.uber.org/fx"

var Module = fx.Module(
	"sso",
	fx.Provide(
		New,
	),
)
