package segments

import (
	"github.com/FreekingDean/gojellyfin/internal/fx"
	"github.com/FreekingDean/gojellyfin/internal/jobs"
)

var Module = fx.Module(
	"segments",
	fx.Provide(New),
	fx.Invoke(register),
)

func register(registry *jobs.Registry, service *Service) {
	registry.Register(service.BatchJob())
	registry.Register(service.SeasonJob())
}
