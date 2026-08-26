package jobs

import (
	"github.com/FreekingDean/gojellyfin/internal/fx"
)

var Module = fx.Module(
	"jobs",
	fx.Provide(
		NewClient,
		NewRegistry,
		NewService,
	),
)

var WorkerModule = fx.Module(
	"jobs/worker",
	fx.Provide(NewWorker),
	fx.InvokeStartStop[*Worker](),
)
