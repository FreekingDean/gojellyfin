package search

import "go.uber.org/fx"

var Module = fx.Module(
	"server/search",
	fx.Provide(
		New,
	),
)
