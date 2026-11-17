package notify

import (
	"go.uber.org/fx"
)

var Module = fx.Module(
	"notify",
	fx.Provide(New),
	fx.Invoke(func(lifecycle fx.Lifecycle, service *Service) {
		lifecycle.Append(fx.StartStopHook(service.Start, service.Stop))
	}),
)
