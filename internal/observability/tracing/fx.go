package tracing

import (
	"github.com/FreekingDean/gojellyfin/internal/fx"
)

var Module = fx.Module(
	"tracing",
	fx.Provide(New),
	fx.InvokeStartStop[*Tracing](),
)
