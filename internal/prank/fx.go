package prank

import (
	"time"

	"go.uber.org/fx"

	"github.com/FreekingDean/gojellyfin/internal/config"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/users"
)

var Module = fx.Module(
	"prank",
	fx.Provide(
		New,
	),
)

func New(catalogue *items.Service, settings *config.Service, administrators *users.Service) *Service {
	return newService(catalogue, settings, administrators, time.Now)
}
