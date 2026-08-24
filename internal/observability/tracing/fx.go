package tracing

import (
	"go.uber.org/fx"
)

var Module = fx.Module(
	"tracing",
	fx.Provide(New),
	fx.Invoke(run),
)

// There is nothing to start: the provider is exporting from the moment it is
// built. Only the flush on the way out needs a hook.
func run(lc fx.Lifecycle, t *Tracing) {
	lc.Append(fx.Hook{
		OnStop: t.Stop,
	})
}
