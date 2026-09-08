package blob

import "github.com/FreekingDean/gojellyfin/internal/fx"

var Module = fx.Module(
	"blob",
	fx.Provide(New),
)
