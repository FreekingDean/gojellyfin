package worker

import (
	"context"
	"errors"
	"fmt"
	"log"
	"maps"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/jobs"
)

var ErrNoRunners = errors.New("worker has no runners")

const (
	defaultConcurrency = 1
	defaultLease       = time.Minute
	defaultPoll        = time.Second
	finishTimeout      = 10 * time.Second
)

type Options struct {
	Concurrency int
	Lease       time.Duration
	Poll        time.Duration
}

type Worker struct {
	jobs    *jobs.Service
	options Options
	name    string
	runners map[string]jobs.Runner

	cancel context.CancelFunc
	group  sync.WaitGroup
}

func New(service *jobs.Service, options Options) *Worker {
	if options.Concurrency <= 0 {
		options.Concurrency = defaultConcurrency
	}
	if options.Lease <= 0 {
		options.Lease = defaultLease
	}
	if options.Poll <= 0 {
		options.Poll = defaultPoll
	}

	return &Worker{
		jobs:    service,
		options: options,
		name:    name(),
		runners: make(map[string]jobs.Runner),
	}
}

func (w *Worker) Handle(kind string, run jobs.Runner) {
	w.runners[kind] = run
}

func (w *Worker) Start() error {
	if len(w.runners) == 0 {
		return ErrNoRunners
	}

	ctx, cancel := context.WithCancel(context.Background())
	w.cancel = cancel

	log.Printf("worker %s handling %v with concurrency %d", w.name, w.kinds(), w.options.Concurrency)
	for range w.options.Concurrency {
		w.group.Add(1)
		go w.loop(ctx)
	}

	return nil
}

func (w *Worker) Stop() error {
	if w.cancel == nil {
		return nil
	}
	w.cancel()
	w.group.Wait()

	return nil
}

func (w *Worker) loop(ctx context.Context) {
	defer w.group.Done()

	for ctx.Err() == nil {
		leased, err := w.next(ctx)
		if err != nil && ctx.Err() == nil {
			log.Printf("worker %s: %v", w.name, err)
		}
		if leased {
			continue
		}

		select {
		case <-ctx.Done():
		case <-time.After(w.options.Poll):
		}
	}
}

func (w *Worker) next(ctx context.Context) (bool, error) {
	job, err := w.jobs.Lease(ctx, w.name, w.kinds(), w.options.Lease)
	if errors.Is(err, jobs.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	w.run(ctx, job)

	return true, nil
}

func (w *Worker) run(ctx context.Context, job *jobs.Job) {
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	progress := &reporter{}
	watch := w.watchLease(runCtx, cancel, job, progress)

	err := w.runners[job.Kind](runCtx, job, progress.report)
	cancel()
	cancelled, lost := watch()

	if lost {
		log.Printf("worker %s lost the lease on job %s", w.name, job.ID)
		return
	}

	finishCtx, finish := context.WithTimeout(context.Background(), finishTimeout)
	defer finish()

	if err := w.record(finishCtx, job, err, cancelled, ctx.Err() != nil); err != nil {
		log.Printf("worker %s: %v", w.name, err)
	}
}

func (w *Worker) record(ctx context.Context, job *jobs.Job, err error, cancelled, draining bool) error {
	switch {
	case cancelled:
		return w.jobs.Cancelled(ctx, job)
	case draining:
		return w.jobs.Release(ctx, job)
	case err != nil:
		log.Printf("job %s (%s) failed: %v", job.ID, job.Kind, err)
		return w.jobs.Fail(ctx, job, err)
	default:
		return w.jobs.Succeed(ctx, job)
	}
}

func (w *Worker) watchLease(ctx context.Context, stop context.CancelFunc, job *jobs.Job, progress *reporter) func() (bool, bool) {
	var cancelled, lost bool
	done := make(chan struct{})

	go func() {
		defer close(done)

		ticker := time.NewTicker(w.options.Lease / 3)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}

			requested, err := w.jobs.Renew(context.Background(), job, w.options.Lease, progress.read())
			if errors.Is(err, jobs.ErrLeaseLost) {
				lost = true
				stop()

				return
			}
			if err != nil {
				log.Printf("worker %s: %v", w.name, err)
				continue
			}
			if requested {
				cancelled = true
				stop()

				return
			}
		}
	}()

	return func() (bool, bool) {
		<-done

		return cancelled, lost
	}
}

func (w *Worker) kinds() []string {
	return slices.Sorted(maps.Keys(w.runners))
}

type reporter struct {
	mu       sync.Mutex
	progress float64
}

func (r *reporter) report(progress float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.progress = progress
}

func (r *reporter) read() float64 {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.progress
}

func name() string {
	host, err := os.Hostname()
	if err != nil {
		host = "unknown"
	}

	return fmt.Sprintf("%s/%d/%s", host, os.Getpid(), uuid.NewString()[:8])
}
