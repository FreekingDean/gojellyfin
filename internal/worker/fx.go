package worker

import (
	"github.com/FreekingDean/gojellyfin/internal/fx"
)

var Module = fx.Module(
	"worker",
	fx.Provide(
		New,
	),
	fx.InvokeStartStop[*Worker](),
)
