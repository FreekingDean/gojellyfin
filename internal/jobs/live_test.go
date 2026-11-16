//go:build temporal

package jobs

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go.temporal.io/api/enums/v1"

	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/observability/tracing"
)

const (
	settle            = 30 * time.Second
	heartbeatThrottle = 90 * time.Second
)

func named(t *testing.T) string {
	t.Helper()

	suffix := make([]byte, 4)
	if _, err := rand.Read(suffix); err != nil {
		t.Fatalf("failed to name the job: %v", err)
	}

	return "Live" + strings.NewReplacer("/", "", "_", "").Replace(t.Name()) + hex.EncodeToString(suffix)
}

func live(t *testing.T, defined ...Job) *Service {
	t.Helper()

	config, err := env.Load()
	if err != nil {
		t.Fatalf("failed to read the environment: %v", err)
	}
	if config.Temporal.HostPort == "" {
		t.Fatal("TEMPORAL_HOSTPORT is not set, and this suite exists to dial a real server")
	}

	connected, err := NewClient(config, &tracing.Tracing{})
	if err != nil {
		t.Fatalf("failed to dial temporal: %v", err)
	}

	registry := NewRegistry()
	for _, job := range defined {
		registry.Register(job)
	}
	service := NewService(connected, registry)

	worker := NewWorker(service, config)
	if err := worker.Start(); err != nil {
		t.Fatalf("the worker did not start: %v", err)
	}
	t.Cleanup(func() {
		for _, job := range defined {
			if job.Startable {
				_ = service.Schedule(context.Background(), job.Name, nil)
				_ = service.Cancel(context.Background(), job.Name)
			}
		}
		_ = worker.Stop()
	})

	return service
}

func eventually(t *testing.T, what string, check func() bool) {
	t.Helper()

	within(t, settle, what, check)
}

func within(t *testing.T, wait time.Duration, what string, check func() bool) {
	t.Helper()

	deadline := time.Now().Add(wait)
	for time.Now().Before(deadline) {
		if check() {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("gave up waiting for %s", what)
}

func state(t *testing.T, service *Service, name string) Status {
	t.Helper()

	status, err := service.Status(context.Background(), name)
	if err != nil {
		t.Fatalf("Status returned %v", err)
	}

	return status
}

func held(started *atomic.Int64, release <-chan struct{}) func(context.Context) error {
	return func(ctx context.Context) error {
		started.Add(1)
		for {
			select {
			case <-release:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(100 * time.Millisecond):
				Heartbeat(ctx)
			}
		}
	}
}

func running(t *testing.T, service *Service, name string) int {
	t.Helper()

	connection, err := service.client.connection()
	if err != nil {
		t.Fatalf("no connection: %v", err)
	}
	found, err := executions(context.Background(), connection, name, runningOnly)
	if err != nil {
		t.Fatalf("failed to list the runs: %v", err)
	}

	return len(found)
}

func TestLive_RunsAJobByItsName(t *testing.T) {
	var ran atomic.Int64
	job := Job{Name: named(t), Startable: true, Run: func(context.Context) error {
		ran.Add(1)

		return nil
	}}
	service := live(t, job)

	if err := service.Start(context.Background(), job.Name); err != nil {
		t.Fatalf("Start returned %v", err)
	}

	eventually(t, "the run to complete", func() bool {
		last := state(t, service, job.Name).Last
		return last != nil && last.Succeeded
	})
	if got := state(t, service, job.Name).State; got != StateIdle {
		t.Errorf("state = %q after the run, want %q", got, StateIdle)
	}
	if ran.Load() != 1 {
		t.Errorf("ran %d times, want once", ran.Load())
	}
}

func TestLive_StartIsASingleton(t *testing.T) {
	var started atomic.Int64
	release := make(chan struct{})
	job := Job{Name: named(t), Startable: true, Run: held(&started, release)}
	service := live(t, job)
	ctx := context.Background()

	for range 3 {
		if err := service.Start(ctx, job.Name); err != nil {
			t.Fatalf("Start returned %v, want a second start to answer success", err)
		}
	}
	eventually(t, "the run to start", func() bool { return started.Load() == 1 })

	if got := running(t, service, job.Name); got != 1 {
		t.Errorf("%d runs are going, want one", got)
	}
	if got := state(t, service, job.Name).State; got != StateRunning {
		t.Errorf("state = %q, want %q", got, StateRunning)
	}

	close(release)
	eventually(t, "the run to finish", func() bool { return state(t, service, job.Name).State == StateIdle })
	if started.Load() != 1 {
		t.Errorf("the job started %d times, want once", started.Load())
	}
}

func TestLive_Cancel(t *testing.T) {
	var started atomic.Int64
	release := make(chan struct{})
	stopping := make(chan struct{})
	job := Job{Name: named(t), Startable: true, Run: func(ctx context.Context) error {
		err := held(&started, release)(ctx)
		<-stopping

		return err
	}}
	service := live(t, job)
	ctx := context.Background()

	if err := service.Start(ctx, job.Name); err != nil {
		t.Fatalf("Start returned %v", err)
	}
	eventually(t, "the run to start", func() bool { return started.Load() == 1 })

	if err := service.Cancel(ctx, job.Name); err != nil {
		t.Fatalf("Cancel returned %v", err)
	}
	eventually(t, "the run to read as cancelling", func() bool { return state(t, service, job.Name).State == StateCancelling })

	close(stopping)
	within(t, heartbeatThrottle, "the run to end", func() bool { return state(t, service, job.Name).State == StateIdle })

	last := state(t, service, job.Name).Last
	if last == nil || !last.Cancelled || last.Succeeded {
		t.Errorf("last = %+v, want a cancelled run", last)
	}
}

func TestLive_Terminated(t *testing.T) {
	var started atomic.Int64
	job := Job{Name: named(t), Startable: true, Run: held(&started, make(chan struct{}))}
	service := live(t, job)
	ctx := context.Background()

	if err := service.Start(ctx, job.Name); err != nil {
		t.Fatalf("Start returned %v", err)
	}
	eventually(t, "the run to start", func() bool { return started.Load() == 1 })

	connection, _ := service.client.connection()
	if err := connection.TerminateWorkflow(ctx, job.Name, "", "the test ends it"); err != nil {
		t.Fatalf("Terminate returned %v", err)
	}

	eventually(t, "the run to end", func() bool { return state(t, service, job.Name).State == StateIdle })
	last := state(t, service, job.Name).Last
	if last == nil || !last.Cancelled {
		t.Errorf("last = %+v, want a terminated run read as cancelled", last)
	}
}

func TestLive_RetriesAFailedAttempt(t *testing.T) {
	var attempts atomic.Int64
	job := Job{Name: named(t), Startable: true, Run: func(context.Context) error {
		if attempts.Add(1) == 1 {
			return errors.New("the first attempt fails")
		}

		return nil
	}}
	service := live(t, job)

	if err := service.Start(context.Background(), job.Name); err != nil {
		t.Fatalf("Start returned %v", err)
	}

	eventually(t, "the retry to succeed", func() bool {
		last := state(t, service, job.Name).Last
		return last != nil && last.Succeeded
	})
	if attempts.Load() != 2 {
		t.Errorf("attempts = %d, want the failure and one retry", attempts.Load())
	}
}

func TestLive_EnqueueFromAJob(t *testing.T) {
	var got atomic.Value
	child := Job{Name: named(t) + "Child", Run: func(ctx context.Context) error {
		value, err := GetParam[string](ctx, "greeting")
		got.Store(value)

		return err
	}}
	parent := Job{Name: named(t), Startable: true, Run: func(ctx context.Context) error {
		return Enqueue(ctx, child.Name, With("greeting", "hello"))
	}}
	service := live(t, parent, child)

	if err := service.Start(context.Background(), parent.Name); err != nil {
		t.Fatalf("Start returned %v", err)
	}

	eventually(t, "the child to run", func() bool { return got.Load() == "hello" })
}

func TestLive_Schedule(t *testing.T) {
	var started atomic.Int64
	release := make(chan struct{})
	job := Job{Name: named(t), Startable: true, Run: held(&started, release)}
	service := live(t, job)
	ctx := context.Background()

	triggers := []Trigger{
		{Kind: TriggerInterval, Interval: 2 * time.Second},
		{Kind: TriggerWeekly, Day: time.Tuesday, TimeOfDay: 4 * time.Hour},
	}
	if err := service.Schedule(ctx, job.Name, triggers); err != nil {
		t.Fatalf("Schedule returned %v", err)
	}

	stored, err := service.Triggers(ctx, job.Name)
	if err != nil {
		t.Fatalf("Triggers returned %v", err)
	}
	if len(stored) != 2 || stored[0] != triggers[0] || stored[1] != triggers[1] {
		t.Errorf("triggers = %+v, want %+v", stored, triggers)
	}

	eventually(t, "the schedule to start a run", func() bool { return started.Load() == 1 })
	if got := state(t, service, job.Name).State; got != StateRunning {
		t.Errorf("state = %q while the scheduled run holds, want %q", got, StateRunning)
	}

	time.Sleep(5 * time.Second)
	if got := running(t, service, job.Name); got != 1 {
		t.Errorf("%d runs are going after the schedule fired again, want the one", got)
	}
	if started.Load() != 1 {
		t.Errorf("started %d times while one run held, want once", started.Load())
	}

	replaced := []Trigger{{Kind: TriggerDaily, TimeOfDay: 3 * time.Hour}}
	if err := service.Schedule(ctx, job.Name, replaced); err != nil {
		t.Fatalf("Schedule returned %v replacing the triggers", err)
	}
	stored, err = service.Triggers(ctx, job.Name)
	if err != nil || len(stored) != 1 || stored[0] != replaced[0] {
		t.Errorf("triggers = %+v, %v, want %+v", stored, err, replaced)
	}

	if err := service.Schedule(ctx, job.Name, nil); err != nil {
		t.Fatalf("Schedule returned %v clearing the triggers", err)
	}
	stored, err = service.Triggers(ctx, job.Name)
	if err != nil || len(stored) != 0 {
		t.Errorf("triggers = %+v, %v, want none once cleared", stored, err)
	}

	close(release)
}

func TestLive_StartupTrigger(t *testing.T) {
	var ran atomic.Int64
	job := Job{Name: named(t), Startable: true, Run: func(context.Context) error {
		ran.Add(1)

		return nil
	}}
	service := live(t, job)
	ctx := context.Background()

	if err := service.Schedule(ctx, job.Name, []Trigger{{Kind: TriggerStartup}}); err != nil {
		t.Fatalf("Schedule returned %v", err)
	}
	if err := service.startOnBoot(ctx); err != nil {
		t.Fatalf("startOnBoot returned %v", err)
	}

	eventually(t, "the startup run", func() bool { return ran.Load() == 1 })
}

func TestLive_Progress(t *testing.T) {
	var remaining atomic.Int64
	remaining.Store(4)
	release := make(chan struct{})
	var started atomic.Int64

	job := Job{
		Name:        named(t),
		Startable:   true,
		Run:         held(&started, release),
		Outstanding: func(context.Context) (int, error) { return int(remaining.Load()), nil },
	}
	service := live(t, job)

	if err := service.Start(context.Background(), job.Name); err != nil {
		t.Fatalf("Start returned %v", err)
	}
	eventually(t, "the run to start", func() bool { return started.Load() == 1 })

	remaining.Store(1)
	close(release)

	eventually(t, "the progress to read", func() bool {
		progress := state(t, service, job.Name).Progress
		return progress != nil && *progress == 75
	})

	remaining.Store(0)
}

func TestLive_StatusOfNothing(t *testing.T) {
	job := Job{Name: named(t), Startable: true, Run: func(context.Context) error { return nil }}
	service := live(t, job)

	status := state(t, service, job.Name)
	if status.State != StateIdle || status.Last != nil || status.Progress != nil {
		t.Errorf("status = %+v, want idle with no history", status)
	}

	if _, err := service.client.client.DescribeTaskQueue(context.Background(), TaskQueue, enums.TASK_QUEUE_TYPE_WORKFLOW); err != nil {
		t.Errorf("the task queue could not be described: %v", err)
	}
}
