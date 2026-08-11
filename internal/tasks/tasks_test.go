package tasks

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestStartRunsToCompletion(t *testing.T) {
	release := make(chan struct{})
	registry := New()
	registry.Register(Definition{ID: "scan", Name: "Scan", Run: func(context.Context) error {
		<-release
		return nil
	}})

	if err := registry.Start("scan"); err != nil {
		t.Fatal(err)
	}
	if got := info(t, registry, "scan").State; got != StateRunning {
		t.Fatalf("got state %q, want %q", got, StateRunning)
	}

	close(release)
	result := waitForIdle(t, registry, "scan")
	if result.Status != StatusCompleted {
		t.Fatalf("got status %q, want %q", result.Status, StatusCompleted)
	}
	if result.EndedAt.Before(result.StartedAt) {
		t.Errorf("ended at %v before started at %v", result.EndedAt, result.StartedAt)
	}
}

func TestStopCancelsRunningTask(t *testing.T) {
	cancelled := make(chan struct{})
	registry := New()
	registry.Register(Definition{ID: "scan", Name: "Scan", Run: func(ctx context.Context) error {
		<-ctx.Done()
		close(cancelled)
		return ctx.Err()
	}})

	if err := registry.Start("scan"); err != nil {
		t.Fatal(err)
	}
	if err := registry.Stop("scan"); err != nil {
		t.Fatal(err)
	}

	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("the task was never cancelled")
	}

	if result := waitForIdle(t, registry, "scan"); result.Status != StatusCancelled {
		t.Fatalf("got status %q, want %q", result.Status, StatusCancelled)
	}
}

func TestStartFailingTask(t *testing.T) {
	registry := New()
	registry.Register(Definition{ID: "scan", Name: "Scan", Run: func(context.Context) error {
		return errors.New("no such directory")
	}})

	if err := registry.Start("scan"); err != nil {
		t.Fatal(err)
	}

	result := waitForIdle(t, registry, "scan")
	if result.Status != StatusFailed {
		t.Fatalf("got status %q, want %q", result.Status, StatusFailed)
	}
	if result.Error != "no such directory" {
		t.Errorf("got error %q, want %q", result.Error, "no such directory")
	}
}

func TestStartWhileRunning(t *testing.T) {
	release := make(chan struct{})
	var runs atomic.Int32
	registry := New()
	registry.Register(Definition{ID: "scan", Name: "Scan", Run: func(context.Context) error {
		runs.Add(1)
		<-release
		return nil
	}})

	for range 2 {
		if err := registry.Start("scan"); err != nil {
			t.Fatal(err)
		}
	}

	close(release)
	waitForIdle(t, registry, "scan")
	if got := runs.Load(); got != 1 {
		t.Fatalf("got %d runs, want 1", got)
	}
}

func TestStopIdleTask(t *testing.T) {
	registry := New()
	registry.Register(Definition{ID: "scan", Name: "Scan", Run: func(context.Context) error { return nil }})

	if err := registry.Stop("scan"); err != nil {
		t.Fatal(err)
	}
	if got := info(t, registry, "scan").State; got != StateIdle {
		t.Fatalf("got state %q, want %q", got, StateIdle)
	}
}

func TestSetTriggers(t *testing.T) {
	registry := New()
	registry.Register(Definition{ID: "scan", Name: "Scan", Run: func(context.Context) error { return nil }})

	day := "Sunday"
	interval := int64(36000000000)
	triggers := []Trigger{{Type: "IntervalTrigger", IntervalTicks: &interval}, {Type: "WeeklyTrigger", DayOfWeek: &day}}
	if err := registry.SetTriggers("scan", triggers); err != nil {
		t.Fatal(err)
	}

	stored := info(t, registry, "scan").Triggers
	if len(stored) != 2 {
		t.Fatalf("got %d triggers, want 2", len(stored))
	}
	if stored[0].Type != "IntervalTrigger" || *stored[0].IntervalTicks != interval {
		t.Errorf("got trigger %+v, want the interval trigger", stored[0])
	}
	if stored[1].Type != "WeeklyTrigger" || *stored[1].DayOfWeek != day {
		t.Errorf("got trigger %+v, want the weekly trigger", stored[1])
	}
}

func TestTasksSortedByName(t *testing.T) {
	registry := New()
	registry.Register(Definition{ID: "scan", Name: "Scan", Category: "Library"})
	registry.Register(Definition{ID: "clean", Name: "Clean", Category: "Maintenance"})

	infos := registry.Tasks()
	if len(infos) != 2 {
		t.Fatalf("got %d tasks, want 2", len(infos))
	}
	if infos[0].Name != "Clean" || infos[1].Name != "Scan" {
		t.Fatalf("got %q then %q, want Clean then Scan", infos[0].Name, infos[1].Name)
	}
	if infos[0].State != StateIdle || infos[0].LastResult != nil {
		t.Errorf("got %+v, want an idle task that never ran", infos[0])
	}
}

func TestUnknownTask(t *testing.T) {
	registry := New()

	if _, err := registry.Task("nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Task: got %v, want ErrNotFound", err)
	}
	if err := registry.Start("nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Start: got %v, want ErrNotFound", err)
	}
	if err := registry.Stop("nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Stop: got %v, want ErrNotFound", err)
	}
	if err := registry.SetTriggers("nope", nil); !errors.Is(err, ErrNotFound) {
		t.Errorf("SetTriggers: got %v, want ErrNotFound", err)
	}
}

func info(t *testing.T, registry *Registry, id string) Info {
	t.Helper()

	found, err := registry.Task(id)
	if err != nil {
		t.Fatal(err)
	}

	return found
}

func waitForIdle(t *testing.T, registry *Registry, id string) Result {
	t.Helper()

	for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); {
		found := info(t, registry, id)
		if found.State == StateIdle && found.LastResult != nil {
			return *found.LastResult
		}
		time.Sleep(time.Millisecond)
	}

	t.Fatalf("task %q never went idle", id)

	return Result{}
}
