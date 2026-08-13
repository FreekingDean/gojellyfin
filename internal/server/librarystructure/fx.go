package librarystructure

import "go.uber.org/fx"

var Module = fx.Module(
	"server/librarystructure",
	fx.Provide(
		New,
	),
)
