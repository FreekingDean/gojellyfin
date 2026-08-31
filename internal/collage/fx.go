package collage

import "github.com/FreekingDean/gojellyfin/internal/fx"

var Module = fx.Module(
	"collage",
	fx.Provide(New),
)
