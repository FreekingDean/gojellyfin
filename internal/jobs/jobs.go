package jobs

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"go.temporal.io/sdk/activity"
	sdktemporal "go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	runWorkflow = "RunJob"
	idSeparator = ":"

	stepTimeout   = 6 * time.Hour
	heartbeat     = 2 * time.Minute
	stepQueued    = 10 * time.Minute
	stepAttempts  = 3
	runTimeoutMax = 24 * time.Hour
)

type Job struct {
	Name        string
	Category    string
	Description string
	Run         func(ctx context.Context) error
}

type Params map[string]string

type Param struct {
	Name  string
	Value string
}

func With[T any](name string, value T) Param {
	encoded, err := json.Marshal(value)
	if err != nil {
		return Param{Name: name, Value: ""}
	}

	return Param{Name: name, Value: string(encoded)}
}

func (p Params) id(name string) string {
	if len(p) == 0 {
		return name
	}

	written := make(url.Values, len(p))
	for key, value := range p {
		written.Set(key, strings.Trim(value, `"`))
	}

	sum := sha1.Sum([]byte(written.Encode()))

	return name + idSeparator + hex.EncodeToString(sum[:])
}

type paramsKey struct{}

type enqueuerKey struct{}

type Enqueuer interface {
	Enqueue(ctx context.Context, name string, params ...Param) error
}

func withJob(ctx context.Context, params Params, enqueuer Enqueuer) context.Context {
	ctx = context.WithValue(ctx, paramsKey{}, params)

	return context.WithValue(ctx, enqueuerKey{}, enqueuer)
}

func GetParam[T any](ctx context.Context, name string) (T, error) {
	var out T

	params, ok := ctx.Value(paramsKey{}).(Params)
	if !ok {
		return out, fmt.Errorf("jobs: %q was asked for outside a job", name)
	}

	raw, held := params[name]
	if !held {
		return out, nil
	}

	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return out, fmt.Errorf("jobs: %q is not a %T: %w", name, out, err)
	}

	return out, nil
}

func Enqueue(ctx context.Context, name string, params ...Param) error {
	enqueuer, ok := ctx.Value(enqueuerKey{}).(Enqueuer)
	if !ok {
		return fmt.Errorf("jobs: %q was enqueued outside a job", name)
	}

	return enqueuer.Enqueue(ctx, name, params...)
}

func Heartbeat(ctx context.Context, detail ...any) {
	if activity.IsActivity(ctx) {
		activity.RecordHeartbeat(ctx, detail...)
	}
}

func run(ctx workflow.Context, name string, params Params) error {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout:    stepTimeout,
		HeartbeatTimeout:       heartbeat,
		ScheduleToStartTimeout: stepQueued,
		RetryPolicy:            &sdktemporal.RetryPolicy{MaximumAttempts: stepAttempts},
	})

	return workflow.ExecuteActivity(ctx, name, params).Get(ctx, nil)
}
