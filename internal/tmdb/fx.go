package tmdb

import (
	"github.com/FreekingDean/gojellyfin/internal/fx"
	"github.com/FreekingDean/gojellyfin/internal/jobs"
)

var Module = fx.Module(
	"tmdb",
	fx.Provide(
		NewClient,
		New,
	),
	fx.Invoke(register),
)

// What the provider is hooked into is answered by reading this file.
func register(registry *jobs.Registry, provider *Provider) {
	registry.Register(NewIdentify(provider))
}
