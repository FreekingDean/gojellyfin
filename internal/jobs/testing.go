package jobs

import (
	"reflect"
	"runtime"
	"strings"
	"testing"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

// A TestEnvironment runs a job's body without a server, so a test can assert
// what the body does with steps that fail. It exists here so that a test, like
// the code it covers, never names the workflow engine.
type TestEnvironment struct {
	env      *testsuite.TestWorkflowEnvironment
	children []string
}

func NewTestEnvironment(t *testing.T) *TestEnvironment {
	t.Helper()

	var suite testsuite.WorkflowTestSuite

	environment := &TestEnvironment{env: suite.NewTestWorkflowEnvironment()}
	environment.env.SetOnChildWorkflowStartedListener(
		func(info *workflow.Info, _ workflow.Context, _ converter.EncodedValues) {
			environment.children = append(environment.children, info.WorkflowExecution.ID)
		},
	)

	return environment
}

// The ids of the nested runs the body started, so a test can say that work was
// chunked rather than fanned out flat.
func (e *TestEnvironment) Children() []string {
	return e.children
}

// ReplaceStep stands a fake in for one of the job's steps. The step is named by
// the function it replaces rather than by a string, so a renamed step is a
// compile error here too.
func (e *TestEnvironment) ReplaceStep(step any, with any) {
	e.env.RegisterActivityWithOptions(with, activity.RegisterOptions{Name: stepName(step)})
}

func RunStep(t *testing.T, step any, args ...any) error {
	t.Helper()

	var suite testsuite.WorkflowTestSuite

	env := suite.NewTestActivityEnvironment()
	env.RegisterActivity(step)
	_, err := env.ExecuteActivity(step, args...)

	return err
}

func (e *TestEnvironment) Run(job Job, options Options) error {
	for _, child := range job.Children() {
		e.env.RegisterWorkflow(child)
	}

	e.env.ExecuteWorkflow(job.Run, options)

	if !e.env.IsWorkflowCompleted() {
		return errNotCompleted
	}

	return e.env.GetWorkflowError()
}

// The engine names an activity after its function, and a method value carries a
// -fm suffix that has to come off for the two to agree.
func stepName(step any) string {
	full := runtime.FuncForPC(reflect.ValueOf(step).Pointer()).Name()
	name := full[strings.LastIndex(full, ".")+1:]

	return strings.TrimSuffix(name, "-fm")
}
