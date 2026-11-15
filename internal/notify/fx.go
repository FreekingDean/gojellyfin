package notify

import (
	"github.com/FreekingDean/gojellyfin/internal/fx"
)

var Module = fx.Module(
	"notify",
	fx.Provide(New),
	fx.InvokeStartStop[*Service](),
)
