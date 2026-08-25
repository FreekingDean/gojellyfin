package jobs

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/mocks"
	"go.temporal.io/sdk/testsuite"
)

var scope = uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8")

type optionsJob struct {
	mutex   sync.Mutex
	ran     []Options
	stepped []Options
	chained []Options
}

func (o *optionsJob) Name() string        { return "OptionsRoundTrip" }
func (o *optionsJob) Category() string    { return "Library" }
func (o *optionsJob) Description() string { return "Records the options it is handed." }
func (o *optionsJob) Steps() []any        { return []any{o.Record} }
func (o *optionsJob) Children() []any     { return []any{o.Chain} }

func (o *optionsJob) Run(ctx Context, options Options) error {
	o.saw(&o.ran, options)

	if err := Step(ctx, o.Record, options).Get(nil); err != nil {
		return err
	}

	return Child(ctx, o.Chain, "chain", options).Get(nil)
}

func (o *optionsJob) Chain(_ Context, options Options) error {
	o.saw(&o.chained, options)

	return nil
}

func (o *optionsJob) Record(_ context.Context, options Options) error {
	o.saw(&o.stepped, options)

	return nil
}

func (o *optionsJob) saw(where *[]Options, options Options) {
	o.mutex.Lock()
	defer o.mutex.Unlock()

	*where = append(*where, options)
}

func TestOptions(t *testing.T) {
	tests := []struct {
		name    string
		options Options
	}{
		{name: "a forced run over one item", options: Options{Force: true, Scope: scope}},
		{name: "an unforced run over everything", options: Options{}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			job := &optionsJob{}

			var suite testsuite.WorkflowTestSuite

			env := suite.NewTestWorkflowEnvironment()
			for _, child := range job.Children() {
				env.RegisterWorkflow(child)
			}
			for _, step := range job.Steps() {
				env.RegisterActivity(step)
			}

			env.ExecuteWorkflow(job.Run, test.options)

			if !env.IsWorkflowCompleted() {
				t.Fatal("the job did not complete")
			}
			if err := env.GetWorkflowError(); err != nil {
				t.Fatalf("the job failed: %v", err)
			}

			for where, seen := range map[string][]Options{
				"the job":   job.ran,
				"its step":  job.stepped,
				"its child": job.chained,
			} {
				if len(seen) != 1 {
					t.Fatalf("%s ran %d times, want once", where, len(seen))
				}
				if seen[0] != test.options {
					t.Errorf("%s saw %+v, want %+v", where, seen[0], test.options)
				}
			}
		})
	}
}

func TestServiceStart(t *testing.T) {
	job := &optionsJob{}
	registry := NewRegistry()
	registry.Register(job)

	t.Run("starts the job under its own name with the options it was given", func(t *testing.T) {
		options := Options{Force: true, Scope: scope}
		connection := &mocks.Client{}
		connection.On("ExecuteWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(nil, nil)

		service := NewService(&Client{client: connection}, registry)
		if err := service.Start(context.Background(), job.Name(), options); err != nil {
			t.Fatalf("the job did not start: %v", err)
		}

		arguments := connection.Calls[0].Arguments
		if started := arguments.Get(1).(client.StartWorkflowOptions); started.ID != job.Name() {
			t.Errorf("workflow id = %q, want the job's name %q", started.ID, job.Name())
		}
		if sent := arguments.Get(3).(Options); sent != options {
			t.Errorf("started with %+v, want %+v", sent, options)
		}
	})

	t.Run("a job already running is not started twice", func(t *testing.T) {
		connection := &mocks.Client{}
		connection.On("ExecuteWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(nil, &serviceerror.WorkflowExecutionAlreadyStarted{})

		service := NewService(&Client{client: connection}, registry)
		if err := service.Start(context.Background(), job.Name(), Options{}); err != nil {
			t.Errorf("a running job refused a second start: %v", err)
		}
	})
}
