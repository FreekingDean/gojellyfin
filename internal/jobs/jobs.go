package jobs

import (
	"context"
	errors "errors"
	"fmt"
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
// without a lock — a second run under a name already running is refused. Steps
// are what its body runs one at a time; Children are the nested bodies it fans
// work out to, and both are declared here so the worker can run them.
type Job interface {
	Name() string
	Description() string
	Category() string
	Run(ctx Context) error
	Steps() []any
	Children() []any
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
	stepQueued    = 10 * time.Minute
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
		// Five heartbeat windows with nobody listening is nobody coming, and the
		// engine will not retry past this by design: what a worker never claimed
		// is re-derived by the next run rather than ambushing the next worker to
		// boot, which is how a scan outlived the run that asked for it.
		ScheduleToStartTimeout: stepQueued,
		// Bounded: work that failed because its source is unreachable will not
		// succeed by being retried inside the same run.
		RetryPolicy: &sdktemporal.RetryPolicy{MaximumAttempts: stepAttempts},
	})

	return future{ctx: ctx, future: workflow.ExecuteActivity(ctx, step, args...)}
}

// Child starts one of the job's nested bodies, which holds its own steps in its
// own history: a body that fanned out thousands of steps would replay all of
// them, and a handful of children each holding a hundred does not.
//
// The id is scoped to the run that started it, so the run before this one
// cannot still hold the name, and a chunk started twice inside one run cannot
// take the name of a sibling.
func Child(ctx Context, child any, name string, args ...any) Future {
	execution := workflow.GetInfo(ctx).WorkflowExecution
	ctx = workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
		WorkflowID:               fmt.Sprintf("%s/%s/%s", execution.ID, execution.RunID, name),
		WorkflowExecutionTimeout: runTimeoutMax,
	})

	return future{ctx: ctx, future: workflow.ExecuteChildWorkflow(ctx, child, args...)}
}

// Logf records against the run rather than the process, and is replay aware, so
// a body can say what it skipped without lying on every replay.
func Logf(ctx Context, message string, args ...any) {
	workflow.GetLogger(ctx).Info(message, args...)
}

// Heartbeat says a step is still moving, so a step that is merely slow is told
// apart from a worker that died. It is a no-op outside a step, so the code a
// step calls into can say where it has got to without knowing who called it.
func Heartbeat(ctx context.Context, detail ...any) {
	if !activity.IsActivity(ctx) {
		return
	}

	activity.RecordHeartbeat(ctx, detail...)
}
