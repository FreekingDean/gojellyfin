package jobs

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

func TestProgress_Percent(t *testing.T) {
	for _, tc := range []struct {
		progress Progress
		want     float64
	}{
		{Progress{StartedWith: 200, Outstanding: 50}, 75},
		{Progress{StartedWith: 10, Outstanding: 40}, 0},
		{Progress{StartedWith: 10, Outstanding: 0}, 100},
	} {
		got := tc.progress.Percent()
		if got == nil || *got != tc.want {
			t.Errorf("%+v = %v, want %v", tc.progress, got, tc.want)
		}
	}

	if got := (Progress{}).Percent(); got != nil {
		t.Errorf("nothing to do = %v, want no percentage", *got)
	}
}

func TestWorker_Run(t *testing.T) {
	var remaining atomic.Int64
	remaining.Store(100)

	job := Job{
		Name:        "ProbeFiles",
		Startable:   true,
		Run:         func(context.Context) error { return nil },
		Outstanding: func(context.Context) (int, error) { return int(remaining.Load()), nil },
	}

	registry := NewRegistry()
	registry.Register(job)
	worker := &Worker{service: NewService(&Client{}, registry)}

	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterWorkflowWithOptions(worker.run, workflow.RegisterOptions{Name: runWorkflow})
	env.RegisterActivityWithOptions(job.Run, activity.RegisterOptions{Name: job.Name})
	env.RegisterActivityWithOptions(job.Outstanding, activity.RegisterOptions{Name: countName(job.Name)})

	seen := []float64{}
	look := func() {
		answered, err := env.QueryWorkflow(progressQuery)
		if err != nil {
			t.Errorf("query failed: %v", err)

			return
		}
		var progress Progress
		if err := answered.Get(&progress); err != nil {
			t.Errorf("progress did not decode: %v", err)

			return
		}
		if percent := progress.Percent(); percent != nil {
			seen = append(seen, *percent)
		}
	}

	env.RegisterDelayedCallback(func() { remaining.Store(60) }, 30*time.Second)
	env.RegisterDelayedCallback(look, 90*time.Second)
	env.RegisterDelayedCallback(func() { remaining.Store(0) }, 2*time.Minute)

	env.ExecuteWorkflow(runWorkflow, job.Name, Params{})

	if !env.IsWorkflowCompleted() {
		t.Fatal("the run never finished")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("the run failed: %v", err)
	}
	if len(seen) != 1 || seen[0] != 40 {
		t.Errorf("progress = %v, want 40%% once forty of a hundred were done", seen)
	}

	t.Run("gives up when nothing moves", func(t *testing.T) {
		remaining.Store(100)

		stuck := suite.NewTestWorkflowEnvironment()
		stuck.RegisterWorkflowWithOptions(worker.run, workflow.RegisterOptions{Name: runWorkflow})
		stuck.RegisterActivityWithOptions(job.Run, activity.RegisterOptions{Name: job.Name})
		stuck.RegisterActivityWithOptions(job.Outstanding, activity.RegisterOptions{Name: countName(job.Name)})

		stuck.ExecuteWorkflow(runWorkflow, job.Name, Params{})

		if !stuck.IsWorkflowCompleted() || stuck.GetWorkflowError() != nil {
			t.Errorf("completed = %v, err = %v, want a run that ends once the count stops falling",
				stuck.IsWorkflowCompleted(), stuck.GetWorkflowError())
		}
	})
}
