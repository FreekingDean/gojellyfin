package prank

import (
	"time"

	"go.uber.org/fx"

	"github.com/FreekingDean/gojellyfin/internal/config"
	"github.com/FreekingDean/gojellyfin/internal/items"
)

var Module = fx.Module(
	"prank",
	fx.Provide(
		New,
	),
)

func New(catalogue *items.Service, settings *config.Service) *Service {
	return newService(catalogue, settings, time.Now)
}
