package collage

import (
	"github.com/FreekingDean/gojellyfin/internal/fx"
	"github.com/FreekingDean/gojellyfin/internal/jobs"
)

var Module = fx.Module(
	"collage",
	fx.Provide(New),
	fx.Invoke(register),
)

func register(registry *jobs.Registry, service *Service) {
	registry.Register(NewLibraryImages(service))
}
