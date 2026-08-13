package playlists

import "go.uber.org/fx"

var Module = fx.Module(
	"server/playlists",
	fx.Provide(
		New,
	),
)
