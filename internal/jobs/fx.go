package jobs

import (
	"go.uber.org/fx"
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
	fx.Invoke(func(lifecycle fx.Lifecycle, worker *Worker) { lifecycle.Append(fx.StartStopHook(worker.Start, worker.Stop)) }),
)
