package jobs

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/store"
	jobmodal "github.com/FreekingDean/gojellyfin/internal/store/job"
	schedulemodal "github.com/FreekingDean/gojellyfin/internal/store/jobschedule"
)

func TestEnqueueDeduplicates(t *testing.T) {
	service, kind := newService(t)

	first, err := service.Enqueue(context.Background(), Request{Kind: kind, DedupeKey: kind})
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Enqueue(context.Background(), Request{Kind: kind, DedupeKey: kind})
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID {
		t.Fatalf("got a second job %s, want the queued %s", second.ID, first.ID)
	}
}

func TestOnlyOneWorkerLeasesADeduplicatedJob(t *testing.T) {
	service, kind := newService(t)

	if _, err := service.Enqueue(context.Background(), Request{Kind: kind, DedupeKey: kind}); err != nil {
		t.Fatal(err)
	}

	leased := make(chan *Job, 2)
	for _, worker := range []string{"one", "two"} {
		go func() {
			job, err := service.Lease(context.Background(), worker, []string{kind}, time.Minute)
			if err != nil {
				leased <- nil
				return
			}
			leased <- job
		}()
	}

	claimed := 0
	for range 2 {
		if job := <-leased; job != nil {
			claimed++
		}
	}
	if claimed != 1 {
		t.Fatalf("got %d workers holding the job, want 1", claimed)
	}
}

func TestEnqueueAfterFinishing(t *testing.T) {
	service, kind := newService(t)
	ctx := context.Background()

	first, err := service.Enqueue(ctx, Request{Kind: kind, DedupeKey: kind})
	if err != nil {
		t.Fatal(err)
	}
	leased, err := service.Lease(ctx, "one", []string{kind}, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Succeed(ctx, leased); err != nil {
		t.Fatal(err)
	}

	second, err := service.Enqueue(ctx, Request{Kind: kind, DedupeKey: kind})
	if err != nil {
		t.Fatal(err)
	}
	if second.ID == first.ID {
		t.Fatal("got the finished job back, want a fresh one")
	}
}

func TestLeaseReclaimsAnExpiredLease(t *testing.T) {
	service, kind := newService(t)
	ctx := context.Background()

	if _, err := service.Enqueue(ctx, Request{Kind: kind, DedupeKey: kind}); err != nil {
		t.Fatal(err)
	}
	abandoned, err := service.Lease(ctx, "dead", []string{kind}, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	expire(t, service, abandoned.ID)

	reclaimed, err := service.Lease(ctx, "alive", []string{kind}, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if reclaimed.ID != abandoned.ID {
		t.Fatalf("got job %s, want the abandoned %s", reclaimed.ID, abandoned.ID)
	}
	if reclaimed.Attempt != abandoned.Attempt+1 {
		t.Errorf("got attempt %d, want %d", reclaimed.Attempt, abandoned.Attempt+1)
	}

	if _, err := service.Renew(ctx, abandoned, time.Minute, 0); !errors.Is(err, ErrLeaseLost) {
		t.Errorf("got %v renewing the stolen lease, want ErrLeaseLost", err)
	}
	if err := service.Succeed(ctx, abandoned); !errors.Is(err, ErrLeaseLost) {
		t.Errorf("got %v finishing the stolen lease, want ErrLeaseLost", err)
	}
}

func TestLeaseSkipsFutureJobsAndOtherKinds(t *testing.T) {
	service, kind := newService(t)
	ctx := context.Background()

	if _, err := service.Enqueue(ctx, Request{Kind: kind, RunAt: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Lease(ctx, "one", []string{kind}, time.Minute); !errors.Is(err, ErrNotFound) {
		t.Errorf("got %v leasing a future job, want ErrNotFound", err)
	}

	if _, err := service.Enqueue(ctx, Request{Kind: kind}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Lease(ctx, "one", []string{kind + "-other"}, time.Minute); !errors.Is(err, ErrNotFound) {
		t.Errorf("got %v leasing another kind, want ErrNotFound", err)
	}
}

func TestCancelRunningJobIsSeenByTheHolder(t *testing.T) {
	service, kind := newService(t)
	ctx := context.Background()

	if _, err := service.Enqueue(ctx, Request{Kind: kind, DedupeKey: kind}); err != nil {
		t.Fatal(err)
	}
	job, err := service.Lease(ctx, "one", []string{kind}, time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	requested, err := service.Renew(ctx, job, time.Minute, 25)
	if err != nil {
		t.Fatal(err)
	}
	if requested {
		t.Fatal("got a cancellation nobody asked for")
	}

	if err := service.Cancel(ctx, job.ID); err != nil {
		t.Fatal(err)
	}
	requested, err = service.Renew(ctx, job, time.Minute, 50)
	if err != nil {
		t.Fatal(err)
	}
	if !requested {
		t.Fatal("the holder never saw the cancellation")
	}

	if err := service.Cancelled(ctx, job); err != nil {
		t.Fatal(err)
	}
	summary := summaryOf(t, service, kind)
	if summary.Active != nil {
		t.Fatalf("got an active job %s, want none", summary.Active.ID)
	}
	if summary.Last == nil || summary.Last.State != StateCancelled {
		t.Fatalf("got %+v, want a cancelled job", summary.Last)
	}
}

func TestCancelQueuedJob(t *testing.T) {
	service, kind := newService(t)
	ctx := context.Background()

	job, err := service.Enqueue(ctx, Request{Kind: kind, DedupeKey: kind})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Cancel(ctx, job.ID); err != nil {
		t.Fatal(err)
	}

	if _, err := service.Lease(ctx, "one", []string{kind}, time.Minute); !errors.Is(err, ErrNotFound) {
		t.Errorf("got %v leasing a cancelled job, want ErrNotFound", err)
	}
	if state := reload(t, service, job.ID).State; state != StateCancelled {
		t.Errorf("got state %q, want %q", state, StateCancelled)
	}
}

func TestFailRetriesUntilTheAttemptsRunOut(t *testing.T) {
	service, kind := newService(t)
	ctx := context.Background()

	if _, err := service.Enqueue(ctx, Request{Kind: kind, DedupeKey: kind, MaxAttempts: 2}); err != nil {
		t.Fatal(err)
	}

	first, err := service.Lease(ctx, "one", []string{kind}, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Fail(ctx, first, errors.New("no such directory")); err != nil {
		t.Fatal(err)
	}

	retried := reload(t, service, first.ID)
	if retried.State != StateQueued {
		t.Fatalf("got state %q, want %q", retried.State, StateQueued)
	}
	if !retried.RunAt.After(time.Now()) {
		t.Errorf("got run at %v, want a backoff into the future", retried.RunAt)
	}

	runNow(t, service, first.ID)
	second, err := service.Lease(ctx, "one", []string{kind}, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Fail(ctx, second, errors.New("no such directory")); err != nil {
		t.Fatal(err)
	}

	failed := reload(t, service, first.ID)
	if failed.State != StateFailed {
		t.Fatalf("got state %q, want %q", failed.State, StateFailed)
	}
	if failed.ErrorMessage != "no such directory" {
		t.Errorf("got error %q, want it kept", failed.ErrorMessage)
	}
	if failed.DedupeKey != "" {
		t.Errorf("got dedupe key %q on a finished job, want it cleared", failed.DedupeKey)
	}
}

func TestReleaseKeepsTheAttemptBudget(t *testing.T) {
	service, kind := newService(t)
	ctx := context.Background()

	if _, err := service.Enqueue(ctx, Request{Kind: kind, DedupeKey: kind}); err != nil {
		t.Fatal(err)
	}
	job, err := service.Lease(ctx, "one", []string{kind}, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Release(ctx, job); err != nil {
		t.Fatal(err)
	}

	released := reload(t, service, job.ID)
	if released.State != StateQueued {
		t.Fatalf("got state %q, want %q", released.State, StateQueued)
	}
	if released.Attempt != job.Attempt-1 {
		t.Errorf("got attempt %d, want %d", released.Attempt, job.Attempt-1)
	}

	again, err := service.Lease(ctx, "two", []string{kind}, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if again.Attempt != job.Attempt {
		t.Errorf("got attempt %d, want %d", again.Attempt, job.Attempt)
	}
}

func TestSummaryReportsTheLibraryScan(t *testing.T) {
	service, _ := newService(t)
	ctx := context.Background()

	summaries, err := service.Summaries(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(summaries) != 1 {
		t.Fatalf("got %d summaries, want 1", len(summaries))
	}
	if summaries[0].Definition.Kind != LibraryScanKind {
		t.Errorf("got kind %q, want %q", summaries[0].Definition.Kind, LibraryScanKind)
	}
	if _, err := service.Summary(ctx, "nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
	if err := service.SetTriggers(ctx, "nope", nil); !errors.Is(err, ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
	if _, err := service.Start(ctx, "nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

func TestTriggersSurviveAService(t *testing.T) {
	service, _ := newService(t)
	ctx := context.Background()
	t.Cleanup(func() {
		_, _ = service.store.JobSchedule.Delete().Where(schedulemodal.Kind(LibraryScanKind)).Exec(context.Background())
	})

	interval := int64(36000000000)
	day := "Sunday"
	stored := []Trigger{{Type: "IntervalTrigger", IntervalTicks: &interval}, {Type: "WeeklyTrigger", DayOfWeek: &day}}
	if err := service.SetTriggers(ctx, LibraryScanKind, stored); err != nil {
		t.Fatal(err)
	}

	reopened := New(service.store)
	summary, err := reopened.Summary(ctx, LibraryScanKind)
	if err != nil {
		t.Fatal(err)
	}
	if len(summary.Triggers) != 2 {
		t.Fatalf("got %d triggers, want 2", len(summary.Triggers))
	}
	if summary.Triggers[0].Type != "IntervalTrigger" || *summary.Triggers[0].IntervalTicks != interval {
		t.Errorf("got %+v, want the interval trigger", summary.Triggers[0])
	}

	if err := reopened.SetTriggers(ctx, LibraryScanKind, nil); err != nil {
		t.Fatal(err)
	}
	summary, err = reopened.Summary(ctx, LibraryScanKind)
	if err != nil {
		t.Fatal(err)
	}
	if len(summary.Triggers) != 0 {
		t.Errorf("got %d triggers, want none", len(summary.Triggers))
	}
}

func newService(t *testing.T) (*Service, string) {
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

	return New(connection.Client()), kind
}

func reload(t *testing.T, service *Service, id uuid.UUID) *Job {
	t.Helper()

	job, err := service.JobByID(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}

	return job
}

func summaryOf(t *testing.T, service *Service, kind string) Summary {
	t.Helper()

	summary, err := service.summary(context.Background(), Definition{Kind: kind})
	if err != nil {
		t.Fatal(err)
	}

	return summary
}

func expire(t *testing.T, service *Service, id uuid.UUID) {
	t.Helper()

	err := service.store.Job.UpdateOneID(id).SetLeaseExpiresAt(time.Now().Add(-time.Hour)).Exec(context.Background())
	if err != nil {
		t.Fatal(err)
	}
}

func runNow(t *testing.T, service *Service, id uuid.UUID) {
	t.Helper()

	if err := service.store.Job.UpdateOneID(id).SetRunAt(time.Now()).Exec(context.Background()); err != nil {
		t.Fatal(err)
	}
}
