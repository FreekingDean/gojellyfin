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

// Only the worker command takes this: the server starts jobs and never runs
// them, which is the whole reason the worker is a separate deployment.
var WorkerModule = fx.Module(
	"jobs/worker",
	fx.Provide(NewWorker),
	fx.InvokeStartStop[*Worker](),
)
