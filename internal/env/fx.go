package env

import (
	"github.com/FreekingDean/gojellyfin/internal/fx"
)

var Module = fx.Module(
	"env",
	fx.Provide(Load),
)
