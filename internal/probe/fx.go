package probe

import (
	"go.uber.org/fx"

	"github.com/FreekingDean/gojellyfin/internal/jobs"
)

var Module = fx.Module(
	"probe",
	fx.Provide(New),
	fx.Invoke(register),
)

func register(registry *jobs.Registry, prober *Prober) {
	registry.Register(prober.Job())
	registry.Register(prober.FileJob())
}
