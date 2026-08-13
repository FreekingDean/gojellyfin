package worker

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/jobs"
	"github.com/FreekingDean/gojellyfin/internal/store"
	jobmodal "github.com/FreekingDean/gojellyfin/internal/store/job"
)

var testOptions = Options{Concurrency: 1, Lease: 900 * time.Millisecond, Poll: 20 * time.Millisecond}

func TestWorkerRunsAQueuedJob(t *testing.T) {
	service, kind := newService(t)
	var runs atomic.Int32

	job := enqueue(t, service, kind)
	start(t, service, kind, func(context.Context, *jobs.Job, jobs.Reporter) error {
		runs.Add(1)

		return nil
	})

	waitForState(t, service, job.ID, jobs.StateSucceeded)
	if got := runs.Load(); got != 1 {
		t.Errorf("got %d runs, want 1", got)
	}
}

func TestTwoWorkersDoNotBothRunADeduplicatedJob(t *testing.T) {
	service, kind := newService(t)
	var runs atomic.Int32

	job := enqueue(t, service, kind)
	if second := enqueue(t, service, kind); second.ID != job.ID {
		t.Fatalf("got a second queued job %s, want the dedupe key to collapse it", second.ID)
	}

	runner := func(ctx context.Context, _ *jobs.Job, _ jobs.Reporter) error {
		runs.Add(1)
		select {
		case <-ctx.Done():
		case <-time.After(300 * time.Millisecond):
		}

		return nil
	}
	start(t, service, kind, runner)
	start(t, service, kind, runner)

	waitForState(t, service, job.ID, jobs.StateSucceeded)
	time.Sleep(200 * time.Millisecond)

	if got := runs.Load(); got != 1 {
		t.Fatalf("got %d runs of a deduplicated job, want 1", got)
	}
}

func TestExpiredLeaseIsReclaimedAndTheJobRunsAgain(t *testing.T) {
	service, kind := newService(t)
	ctx := context.Background()

	job := enqueue(t, service, kind)
	abandoned, err := service.Lease(ctx, "dead", []string{kind}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	expire(t, service, abandoned.ID)

	var runs atomic.Int32
	start(t, service, kind, func(context.Context, *jobs.Job, jobs.Reporter) error {
		runs.Add(1)

		return nil
	})

	waitForState(t, service, job.ID, jobs.StateSucceeded)
	if got := runs.Load(); got != 1 {
		t.Errorf("got %d runs of the reclaimed job, want 1", got)
	}
	if err := service.Succeed(ctx, abandoned); !errors.Is(err, jobs.ErrLeaseLost) {
		t.Errorf("got %v finishing the abandoned lease, want ErrLeaseLost", err)
	}
}

func TestCancelledJobStopsAndReportsCancelled(t *testing.T) {
	service, kind := newService(t)
	running := make(chan struct{})
	stopped := make(chan struct{})

	job := enqueue(t, service, kind)
	start(t, service, kind, func(ctx context.Context, _ *jobs.Job, _ jobs.Reporter) error {
		close(running)
		<-ctx.Done()
		close(stopped)

		return ctx.Err()
	})

	<-running
	if err := service.Cancel(context.Background(), job.ID); err != nil {
		t.Fatal(err)
	}

	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("the job never stopped")
	}
	waitForState(t, service, job.ID, jobs.StateCancelled)
}

func TestJobSurvivesAWorkerRestart(t *testing.T) {
	service, kind := newService(t)
	running := make(chan struct{}, 2)
	var runs atomic.Int32

	job := enqueue(t, service, kind)
	first := New(service, testOptions)
	first.Handle(kind, func(ctx context.Context, _ *jobs.Job, _ jobs.Reporter) error {
		running <- struct{}{}
		if runs.Add(1) == 1 {
			<-ctx.Done()

			return ctx.Err()
		}

		return nil
	})
	if err := first.Start(); err != nil {
		t.Fatal(err)
	}

	<-running
	if err := first.Stop(); err != nil {
		t.Fatal(err)
	}

	released := reload(t, service, job.ID)
	if released.State != jobs.StateQueued {
		t.Fatalf("got state %q after the worker stopped, want %q", released.State, jobs.StateQueued)
	}
	if released.Attempt != 0 {
		t.Errorf("got attempt %d, want the drained attempt refunded", released.Attempt)
	}

	start(t, service, kind, func(context.Context, *jobs.Job, jobs.Reporter) error {
		running <- struct{}{}
		runs.Add(1)

		return nil
	})
	waitForState(t, service, job.ID, jobs.StateSucceeded)
}

func TestProgressReachesTheQueue(t *testing.T) {
	service, kind := newService(t)
	release := make(chan struct{})

	job := enqueue(t, service, kind)
	start(t, service, kind, func(_ context.Context, _ *jobs.Job, report jobs.Reporter) error {
		report(42)
		<-release

		return nil
	})

	deadline := time.Now().Add(5 * time.Second)
	for reload(t, service, job.ID).Progress != 42 {
		if time.Now().After(deadline) {
			close(release)
			t.Fatal("the reported progress never reached the queue")
		}
		time.Sleep(20 * time.Millisecond)
	}
	close(release)
	waitForState(t, service, job.ID, jobs.StateSucceeded)
}

func TestFailedJobStaysVisible(t *testing.T) {
	service, kind := newService(t)

	job, err := service.Enqueue(context.Background(), jobs.Request{Kind: kind, DedupeKey: kind, MaxAttempts: 1})
	if err != nil {
		t.Fatal(err)
	}
	start(t, service, kind, func(context.Context, *jobs.Job, jobs.Reporter) error {
		return errors.New("no such directory")
	})

	waitForState(t, service, job.ID, jobs.StateFailed)
	if got := reload(t, service, job.ID).ErrorMessage; got != "no such directory" {
		t.Errorf("got error %q, want it kept on the failed job", got)
	}
}

func TestStartWithoutRunners(t *testing.T) {
	service, _ := newService(t)

	if err := New(service, testOptions).Start(); !errors.Is(err, ErrNoRunners) {
		t.Errorf("got %v, want ErrNoRunners", err)
	}
}

func newService(t *testing.T) (*jobs.Service, string) {
	t.Helper()

	connection, err := store.NewStore()
	if err != nil {
		t.Fatalf("failed to open the database: %v", err)
	}
	if err := connection.Start(); err != nil {
		t.Fatalf("failed to reach the database, set DATABASE_URL: %v", err)
	}

	kind := t.Name() + "-" + uuid.NewString()
	t.Cleanup(func() {
		ctx := context.Background()
		if _, err := connection.Client().Job.Delete().Where(jobmodal.Kind(kind)).Exec(ctx); err != nil {
			t.Errorf("failed to delete the jobs: %v", err)
		}
		if err := connection.Stop(); err != nil {
			t.Errorf("failed to close the database: %v", err)
		}
	})

	return jobs.New(connection.Client()), kind
}

func start(t *testing.T, service *jobs.Service, kind string, run jobs.Runner) *Worker {
	t.Helper()

	worker := New(service, testOptions)
	worker.Handle(kind, run)
	if err := worker.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := worker.Stop(); err != nil {
			t.Errorf("failed to stop the worker: %v", err)
		}
	})

	return worker
}

func enqueue(t *testing.T, service *jobs.Service, kind string) *jobs.Job {
	t.Helper()

	job, err := service.Enqueue(context.Background(), jobs.Request{Kind: kind, DedupeKey: kind})
	if err != nil {
		t.Fatal(err)
	}

	return job
}

func reload(t *testing.T, service *jobs.Service, id uuid.UUID) *jobs.Job {
	t.Helper()

	job, err := service.JobByID(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}

	return job
}

func waitForState(t *testing.T, service *jobs.Service, id uuid.UUID, want jobs.State) {
	t.Helper()

	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); {
		if state := reload(t, service, id).State; state == want {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}

	t.Fatalf("job %s never reached %q, it is %q", id, want, reload(t, service, id).State)
}

func expire(t *testing.T, service *jobs.Service, id uuid.UUID) {
	t.Helper()

	if _, err := service.Renew(context.Background(), reload(t, service, id), -time.Hour, 0); err != nil {
		t.Fatal(err)
	}
}
