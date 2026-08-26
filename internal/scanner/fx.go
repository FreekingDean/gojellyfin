package scanner

import (
	"github.com/FreekingDean/gojellyfin/internal/fx"
	"github.com/FreekingDean/gojellyfin/internal/jobs"
)

var Module = fx.Module(
	"scanner",
	fx.Provide(New),
	fx.Invoke(register),
)

func register(registry *jobs.Registry, scanner *Scanner) {
	registry.Register(NewLibraryScan(scanner))
}
