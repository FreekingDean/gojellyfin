package ffmpeg

import "go.uber.org/fx"

var Module = fx.Module(
	"ffmpeg",
	fx.Provide(New),
)
