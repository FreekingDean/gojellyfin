package tracing

import (
	"go.uber.org/fx"
)

var Module = fx.Module(
	"tracing",
	fx.Provide(New),
	fx.Invoke(run),
)

func run(lc fx.Lifecycle, t *Tracing) {
	lc.Append(fx.Hook{
		OnStop: t.Stop,
	})
}
