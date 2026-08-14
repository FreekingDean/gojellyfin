package jobs

import (
	"context"
	errors "errors"
	"time"

	"go.temporal.io/sdk/activity"

	sdktemporal "go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// Context is what a job's body is handed. It is aliased so that nothing outside
// this package imports the workflow engine: a job is written against Job,
// Context and Step, and the engine is an implementation detail of this package.
var errNotCompleted = errors.New("jobs: the job did not complete")

type Context = workflow.Context

// A Job is one unit of background work. Name is the id the dashboard drives it
// by and the id the engine runs it under, which is what makes a job a singleton
// without a lock — a second run under a name already running is refused.
type Job interface {
	Name() string
	Description() string
	Category() string
	Run(ctx Context) error
	Steps() []any
}

// A Future is a step that has been started and not yet waited on, so a body can
// start several and collect them afterwards.
type Future interface {
	Get(out any) error
}

type future struct {
	ctx    Context
	future workflow.Future
}

func (f future) Get(out any) error {
	return f.future.Get(f.ctx, out)
}

const (
	stepTimeout   = 6 * time.Hour
	heartbeat     = 2 * time.Minute
	stepAttempts  = 3
	runTimeoutMax = 24 * time.Hour
)

// Step starts one of the job's steps. The step is named by its function rather
// than by a string, so a renamed method is a compile error instead of a job
// that fails at run time looking for something that no longer exists.
func Step(ctx Context, step any, args ...any) Future {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: stepTimeout,
		HeartbeatTimeout:    heartbeat,
		// Bounded: work that failed because its source is unreachable will not
		// succeed by being retried inside the same run.
		RetryPolicy: &sdktemporal.RetryPolicy{MaximumAttempts: stepAttempts},
	})

	return future{ctx: ctx, future: workflow.ExecuteActivity(ctx, step, args...)}
}

// Logf records against the run rather than the process, and is replay aware, so
// a body can say what it skipped without lying on every replay.
func Logf(ctx Context, message string, args ...any) {
	workflow.GetLogger(ctx).Info(message, args...)
}

// Heartbeat says a step is still moving, so a step that is merely slow is told
// apart from a worker that died.
func Heartbeat(ctx context.Context, detail ...any) {
	activity.RecordHeartbeat(ctx, detail...)
}
