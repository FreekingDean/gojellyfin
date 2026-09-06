package jobs

import (
	"fmt"
	"log"
	"runtime"

	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

type Worker struct {
	client   *Client
	registry *Registry
	workers  map[string]worker.Worker
}

func NewWorker(client *Client, registry *Registry) *Worker {
	return &Worker{client: client, registry: registry}
}

func (w *Worker) Start() error {
	connection, err := w.client.connection()
	if err != nil {
		return err
	}

	w.workers = make(map[string]worker.Worker)
	for _, job := range w.registry.All() {
		polling, known := w.workers[job.Queue()]
		if !known {
			polling = worker.New(connection, job.Queue(), worker.Options{
				MaxConcurrentActivityExecutionSize: runtime.GOMAXPROCS(0),
			})
			w.workers[job.Queue()] = polling
		}

		polling.RegisterWorkflowWithOptions(job.Run, workflow.RegisterOptions{Name: job.Name()})
		for _, child := range job.Children() {
			polling.RegisterWorkflow(child)
		}
		for _, step := range job.Steps() {
			polling.RegisterActivity(step)
		}
		log.Printf("registered job %s on queue %s", job.Name(), job.Queue())
	}

	for queue, polling := range w.workers {
		if err := polling.Start(); err != nil {
			_ = w.Stop()

			return fmt.Errorf("jobs: worker for queue %s: %w", queue, err)
		}
	}

	return nil
}

func (w *Worker) Stop() error {
	for _, polling := range w.workers {
		polling.Stop()
	}
	w.client.Close()

	return nil
}
