package sources

import (
	"go.uber.org/fx"

	"github.com/FreekingDean/gojellyfin/internal/jobs"
)

var Module = fx.Module(
	"sources",
	fx.Provide(
		New,
	),
	fx.Invoke(register),
)

func register(registry *jobs.Registry, service *Service) {
	registry.Register(service.RefreshJob())
}
