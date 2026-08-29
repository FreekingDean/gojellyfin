package artwork

import "github.com/FreekingDean/gojellyfin/internal/fx"

var Module = fx.Module(
	"artwork",
	fx.Provide(New),
)
