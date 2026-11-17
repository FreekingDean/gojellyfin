package metadata

import (
	"go.uber.org/fx"

	"github.com/FreekingDean/gojellyfin/internal/discover"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/jobs"
	"github.com/FreekingDean/gojellyfin/internal/localization"
	"github.com/FreekingDean/gojellyfin/internal/metadata/tmdb"
)

var Module = fx.Module(
	"metadata",
	tmdb.Module,
	fx.Provide(
		provider,
		discoverable,
		ratings,
		New,
	),
	fx.Invoke(register),
)

func provider(client *tmdb.Client) Provider {
	return client
}

func ratings(service *localization.Service) items.RatingLevels {
	return service
}

func discoverable(client *tmdb.Client) discover.Provider {
	return client
}

func register(registry *jobs.Registry, service *Service) {
	registry.Register(service.Job())
	registry.Register(service.ItemJob())
}
