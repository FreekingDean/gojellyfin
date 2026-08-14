package jobs

import (
	"reflect"
	"runtime"
	"strings"
	"testing"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"
)

// A TestEnvironment runs a job's body without a server, so a test can assert
// what the body does with steps that fail. It exists here so that a test, like
// the code it covers, never names the workflow engine.
type TestEnvironment struct {
	env *testsuite.TestWorkflowEnvironment
}

func NewTestEnvironment(t *testing.T) *TestEnvironment {
	t.Helper()

	var suite testsuite.WorkflowTestSuite

	return &TestEnvironment{env: suite.NewTestWorkflowEnvironment()}
}

// ReplaceStep stands a fake in for one of the job's steps. The step is named by
// the function it replaces rather than by a string, so a renamed step is a
// compile error here too.
func (e *TestEnvironment) ReplaceStep(step any, with any) {
	e.env.RegisterActivityWithOptions(with, activity.RegisterOptions{Name: stepName(step)})
}

// RunStep runs one step the way the engine would. A step that heartbeats needs
// an activity context to do it on, so calling the method directly panics where
// this does not.
func RunStep(t *testing.T, step any, args ...any) error {
	t.Helper()

	var suite testsuite.WorkflowTestSuite

	env := suite.NewTestActivityEnvironment()
	env.RegisterActivity(step)
	_, err := env.ExecuteActivity(step, args...)

	return err
}

func (e *TestEnvironment) Run(job Job) error {
	e.env.ExecuteWorkflow(job.Run)

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
