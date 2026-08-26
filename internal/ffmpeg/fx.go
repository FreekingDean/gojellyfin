package ffmpeg

import "github.com/FreekingDean/gojellyfin/internal/fx"

var Module = fx.Module(
	"ffmpeg",
	fx.Provide(New),
)
