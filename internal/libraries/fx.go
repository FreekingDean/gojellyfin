package libraries

import (
	"go.uber.org/fx"

	"github.com/FreekingDean/gojellyfin/internal/jobs"
	"github.com/FreekingDean/gojellyfin/internal/sources"
)

var Module = fx.Module(
	"libraries",
	fx.Provide(
		New,
	),
	fx.Invoke(register),
)

func register(lifecycle fx.Lifecycle, registry *jobs.Registry, service *Service, bindings *sources.Service) {
	lifecycle.Append(fx.Hook{OnStart: service.EnsureFixed})
	service.UseSources(bindings)
	registry.Register(service.RefreshLibrariesJob())
	registry.Register(service.RefreshLibraryJob())
}
