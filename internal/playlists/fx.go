package playlists

import "go.uber.org/fx"

var Module = fx.Module(
	"playlists",
	fx.Provide(
		New,
	),
)
