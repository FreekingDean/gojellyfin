package metadata

import (
	"github.com/FreekingDean/gojellyfin/internal/fx"
	"github.com/FreekingDean/gojellyfin/internal/jobs"
	"github.com/FreekingDean/gojellyfin/internal/tmdb"
)

// Which provider is asked is settled here and nowhere else, so a command names
// this module and stays ignorant of who answers.
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

// What the provider is hooked into is answered by reading this file.
func register(registry *jobs.Registry, service *Service) {
	registry.Register(NewIdentify(service))
}
