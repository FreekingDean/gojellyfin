package jobs

import (
	"context"
	"log"
	"runtime"
	"sync"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"

	"github.com/FreekingDean/gojellyfin/internal/env"
)

const bootTimeout = 30 * time.Second

type Worker struct {
	service     *Service
	stopTimeout time.Duration
	workers     []worker.Worker
}

func NewWorker(service *Service, config env.Config) *Worker {
	return &Worker{service: service, stopTimeout: config.Temporal.WorkerStopTimeout}
}

func (w *Worker) Start() error {
	connection, err := w.service.client.connection()
	if err != nil {
		return err
	}

	for _, queue := range w.service.registry.Queues() {
		polling := worker.New(connection, queue, worker.Options{
			MaxConcurrentActivityExecutionSize: runtime.GOMAXPROCS(0),
			WorkerStopTimeout:                  w.stopTimeout,
		})
		polling.RegisterWorkflowWithOptions(w.run, workflow.RegisterOptions{Name: runWorkflow})
		polling.RegisterWorkflowWithOptions(w.schedule, workflow.RegisterOptions{Name: scheduleWorkflow})

		for _, job := range w.service.registry.All() {
			polling.RegisterActivityWithOptions(
				w.activity(job),
				activity.RegisterOptions{Name: job.Name},
			)
			if job.Outstanding != nil {
				polling.RegisterActivityWithOptions(job.Outstanding, activity.RegisterOptions{Name: countName(job.Name)})
			}
		}

		if err := polling.Start(); err != nil {
			return err
		}
		w.workers = append(w.workers, polling)
		log.Printf("polling task queue %s", queue)
	}

	for _, job := range w.service.registry.All() {
		log.Printf("registered job %s on %s", job.Name, job.queue())
	}

	booting, cancel := context.WithTimeout(context.Background(), bootTimeout)
	defer cancel()
	if err := w.service.startOnBoot(booting); err != nil {
		log.Printf("failed to start the jobs that run at startup: %v", err)
	}

	return nil
}

func (w *Worker) activity(job Job) func(context.Context, Params) error {
	return func(ctx context.Context, params Params) error {
		return job.Run(withJob(ctx, params, w.service))
	}
}

func (w *Worker) Stop() error {
	var stopping sync.WaitGroup
	for _, polling := range w.workers {
		stopping.Go(polling.Stop)
	}
	stopping.Wait()
	w.service.client.Close()

	return nil
}
