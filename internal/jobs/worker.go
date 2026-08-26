package jobs

import (
	"log"
	"runtime"

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
	for _, job := range w.registry.All() {
		w.worker.RegisterWorkflowWithOptions(job.Run, workflow.RegisterOptions{Name: job.Name()})
		for _, child := range job.Children() {
			w.worker.RegisterWorkflow(child)
		}
		for _, step := range job.Steps() {
			w.worker.RegisterActivity(step)
		}
		log.Printf("registered job %s", job.Name())
	}

	return w.worker.Start()
}

func (w *Worker) Stop() error {
	if w.worker != nil {
		w.worker.Stop()
	}
	w.client.Close()

	return nil
}
