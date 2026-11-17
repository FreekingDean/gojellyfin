package tmdb

import "go.uber.org/fx"

var Module = fx.Module(
	"tmdb",
	fx.Provide(NewClient),
)
