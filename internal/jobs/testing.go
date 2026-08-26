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

func (e *TestEnvironment) Children() []string {
	return e.children
}

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

func stepName(step any) string {
	full := runtime.FuncForPC(reflect.ValueOf(step).Pointer()).Name()
	name := full[strings.LastIndex(full, ".")+1:]

	return strings.TrimSuffix(name, "-fm")
}
