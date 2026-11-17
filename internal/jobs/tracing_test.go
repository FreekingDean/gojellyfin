package jobs

import (
	"context"
	"slices"
	"strings"
	"testing"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/interceptor"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"

	"github.com/FreekingDean/gojellyfin/internal/observability/tracing"
)

func TestTracer_SpansOnlyTheWork(t *testing.T) {
	traces, recorder := tracing.Recorded()
	traced, err := tracer(traces)
	if err != nil {
		t.Fatal(err)
	}

	job := Job{Name: "ProbeFile", Run: func(context.Context) error { return nil }}
	registry := NewRegistry()
	registry.Register(job)
	runner := &Worker{service: NewService(&Client{}, registry)}

	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.SetWorkerOptions(worker.Options{Interceptors: []interceptor.WorkerInterceptor{traced}})
	env.RegisterWorkflowWithOptions(runner.run, workflow.RegisterOptions{Name: runWorkflow})
	env.RegisterActivityWithOptions(runner.activity(job), activity.RegisterOptions{Name: job.Name})

	env.ExecuteWorkflow(runWorkflow, job.Name, Params{"file": `"x"`})
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("the run failed: %v", err)
	}

	names := recorder.Names()
	if !slices.Contains(names, "RunActivity:ProbeFile") {
		t.Errorf("spans = %v, want the activity's", names)
	}
	for _, name := range names {
		if strings.HasPrefix(name, "RunWorkflow:") || strings.HasPrefix(name, "StartActivity:") {
			t.Errorf("recorded %q, which spans the time the job sat queued", name)
		}
	}
	if _, ok := recorder.Values()["job.queued_ms"]; !ok {
		t.Errorf("attributes = %v, want how long the job was queued", recorder.Values())
	}
}
