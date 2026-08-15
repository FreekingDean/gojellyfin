package tmdb

import "github.com/FreekingDean/gojellyfin/internal/fx"

var Module = fx.Module(
	"tmdb",
	fx.Provide(NewClient),
)
