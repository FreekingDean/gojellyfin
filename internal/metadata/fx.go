package metadata

import (
	"github.com/FreekingDean/gojellyfin/internal/fx"
	"github.com/FreekingDean/gojellyfin/internal/jobs"
	"github.com/FreekingDean/gojellyfin/internal/metadata/tmdb"
)

var Module = fx.Module(
	"metadata",
	tmdb.Module,
	fx.Provide(
		provider,
		New,
	),
	fx.Invoke(register),
)

func provider(client *tmdb.Client) Provider {
	return client
}

func register(registry *jobs.Registry, service *Service) {
	registry.Register(NewIdentify(service))
}
