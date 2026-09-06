package jobs

import (
	"context"
	errors "errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"go.temporal.io/sdk/activity"

	sdktemporal "go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

var errNotCompleted = errors.New("jobs: the job did not complete")

type Context = workflow.Context

type Options struct {
	Force bool
	Scope uuid.UUID
}

type Job interface {
	Name() string
	Description() string
	Category() string
	Queue() string
	Run(ctx Context, options Options) error
	Steps() []any
	Children() []any
}

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

func Step(ctx Context, step any, args ...any) Future {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout:    stepTimeout,
		HeartbeatTimeout:       heartbeat,
		ScheduleToStartTimeout: stepQueued,
		RetryPolicy:            &sdktemporal.RetryPolicy{MaximumAttempts: stepAttempts},
	})

	return future{ctx: ctx, future: workflow.ExecuteActivity(ctx, step, args...)}
}

func Child(ctx Context, child any, name string, args ...any) Future {
	execution := workflow.GetInfo(ctx).WorkflowExecution
	ctx = workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
		WorkflowID:               fmt.Sprintf("%s/%s/%s", execution.ID, execution.RunID, name),
		WorkflowExecutionTimeout: runTimeoutMax,
	})

	return future{ctx: ctx, future: workflow.ExecuteChildWorkflow(ctx, child, args...)}
}

func Logf(ctx Context, message string, args ...any) {
	workflow.GetLogger(ctx).Info(message, args...)
}

func Heartbeat(ctx context.Context, detail ...any) {
	if !activity.IsActivity(ctx) {
		return
	}

	activity.RecordHeartbeat(ctx, detail...)
}
