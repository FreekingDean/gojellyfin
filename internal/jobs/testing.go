package jobs

import (
	"context"
	"testing"

	"go.temporal.io/sdk/testsuite"
)

type queued struct {
	Name   string
	Params Params
}

type recorder struct {
	enqueued []queued
}

func (r *recorder) Enqueue(_ context.Context, name string, params ...Param) error {
	held := make(Params, len(params))
	for _, param := range params {
		held[param.Name] = param.Value
	}
	r.enqueued = append(r.enqueued, queued{Name: name, Params: held})

	return nil
}

func RunJob(t *testing.T, job Job, params ...Param) ([]queued, error) {
	t.Helper()

	held := make(Params, len(params))
	for _, param := range params {
		held[param.Name] = param.Value
	}

	written := &recorder{}

	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestActivityEnvironment()

	activity := func(ctx context.Context, given Params) error {
		return job.Run(withJob(ctx, given, written))
	}
	env.RegisterActivity(activity)

	if _, err := env.ExecuteActivity(activity, held); err != nil {
		return written.enqueued, err
	}

	return written.enqueued, nil
}

func Enqueued(t *testing.T, from []queued, name string) []Params {
	t.Helper()

	found := make([]Params, 0, len(from))
	for _, one := range from {
		if one.Name == name {
			found = append(found, one.Params)
		}
	}

	return found
}
