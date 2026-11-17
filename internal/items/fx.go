package items

import (
	"go.uber.org/fx"

	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/jobs"
)

var Module = fx.Module(
	"items",
	fx.Provide(
		New,
	),
	fx.Invoke(register, limit),
)

func register(registry *jobs.Registry, service *Service) {
	registry.Register(service.RefreshItemJob())
	registry.Register(service.SweepJob())
}

func limit(config env.Config, service *Service) {
	service.EncodeAtMost(config.Transcoder.MaxSourceHeight)
}
