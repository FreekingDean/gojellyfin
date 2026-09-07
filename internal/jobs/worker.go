package jobs

import (
	"context"
	"log"
	"runtime"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

type Worker struct {
	client   *Client
	registry *Registry
	worker   worker.Worker
}

func NewWorker(client *Client, registry *Registry) *Worker {
	return &Worker{client: client, registry: registry}
}

func (w *Worker) Start() error {
	connection, err := w.client.connection()
	if err != nil {
		return err
	}

	w.worker = worker.New(connection, TaskQueue, worker.Options{
		MaxConcurrentActivityExecutionSize: runtime.GOMAXPROCS(0),
	})
	w.worker.RegisterWorkflowWithOptions(run, workflow.RegisterOptions{Name: runWorkflow})

	for _, job := range w.registry.All() {
		w.worker.RegisterActivityWithOptions(
			w.activity(job),
			activity.RegisterOptions{Name: job.Name},
		)
		log.Printf("registered job %s", job.Name)
	}

	return w.worker.Start()
}

func (w *Worker) activity(job Job) func(context.Context, Params) error {
	return func(ctx context.Context, params Params) error {
		return job.Run(withJob(ctx, params, w.client))
	}
}

func (w *Worker) Stop() error {
	if w.worker != nil {
		w.worker.Stop()
	}
	w.client.Close()

	return nil
}
