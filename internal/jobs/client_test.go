package jobs

import (
	"context"
	"errors"
	"strings"
	"testing"

	"go.temporal.io/api/common/v1"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/api/workflow/v1"
	"go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/client"

	"github.com/FreekingDean/gojellyfin/internal/env"
)

func TestNewClient(t *testing.T) {
	t.Run("without an address is disabled", func(t *testing.T) {
		client, err := NewClient(env.Config{})
		if err != nil {
			t.Fatalf("an unconfigured client failed to build: %v", err)
		}
		if client.Enabled() {
			t.Error("a client with no address reports itself enabled")
		}
	})

	t.Run("requires a namespace", func(t *testing.T) {
		_, err := NewClient(env.Config{Temporal: env.Temporal{HostPort: "temporal:7233"}})

		if !errors.Is(err, ErrNoNamespace) {
			t.Errorf("err = %v, want ErrNoNamespace", err)
		}
	})
}

type queue struct {
	client.Client
	queries   []string
	running   []*workflow.WorkflowExecutionInfo
	finished  []*workflow.WorkflowExecutionInfo
	cancelled []string
}

func (q *queue) ListWorkflow(
	_ context.Context,
	request *workflowservice.ListWorkflowExecutionsRequest,
) (*workflowservice.ListWorkflowExecutionsResponse, error) {
	q.queries = append(q.queries, request.GetQuery())

	found := q.finished
	if strings.Contains(request.GetQuery(), runningOnly) {
		found = q.running
	}

	return &workflowservice.ListWorkflowExecutionsResponse{Executions: found}, nil
}

func (q *queue) DescribeWorkflowExecution(
	_ context.Context,
	id string,
	_ string,
) (*workflowservice.DescribeWorkflowExecutionResponse, error) {
	return nil, serviceerror.NewNotFound(id)
}

func (q *queue) CancelWorkflow(_ context.Context, id string, _ string) error {
	q.cancelled = append(q.cancelled, id)

	return nil
}

type refreshJob struct{}

func (refreshJob) Name() string              { return "RefreshMetadata" }
func (refreshJob) Category() string          { return "Library" }
func (refreshJob) Description() string       { return "Fetches metadata." }
func (refreshJob) Run(context.Context) error { return nil }

func serviceOver(t *testing.T, queued *queue) *Service {
	t.Helper()

	registry := NewRegistry()
	registry.Register(refreshJob{})

	return NewService(&Client{client: queued}, registry)
}

func execution(id string, status enums.WorkflowExecutionStatus) *workflow.WorkflowExecutionInfo {
	return &workflow.WorkflowExecutionInfo{
		Execution: &common.WorkflowExecution{WorkflowId: id},
		Status:    status,
	}
}

func TestService_Status(t *testing.T) {
	scoped := Params{"force": "false", "scope": "b3f"}.id("RefreshMetadata")

	t.Run("sees a run started with params", func(t *testing.T) {
		queued := &queue{
			running: []*workflow.WorkflowExecutionInfo{
				execution(scoped, enums.WORKFLOW_EXECUTION_STATUS_RUNNING),
			},
		}

		status, err := serviceOver(t, queued).Status(context.Background(), "RefreshMetadata")
		if err != nil {
			t.Fatalf("Status returned %v", err)
		}
		if status.State != StateRunning {
			t.Errorf("state = %q, want %q while %q runs", status.State, StateRunning, scoped)
		}
	})

	t.Run("asks for the job and the ids under it", func(t *testing.T) {
		queued := &queue{}

		if _, err := serviceOver(t, queued).Status(context.Background(), "RefreshMetadata"); err != nil {
			t.Fatalf("Status returned %v", err)
		}
		if len(queued.queries) == 0 {
			t.Fatal("nothing was asked of the queue")
		}
		for _, want := range []string{`WorkflowId = "RefreshMetadata"`, `WorkflowId STARTS_WITH "RefreshMetadata:"`} {
			if !strings.Contains(queued.queries[0], want) {
				t.Errorf("query = %q, want it to name %s", queued.queries[0], want)
			}
		}
	})

	t.Run("reports the last result of a run started with params", func(t *testing.T) {
		queued := &queue{
			finished: []*workflow.WorkflowExecutionInfo{
				execution(scoped, enums.WORKFLOW_EXECUTION_STATUS_COMPLETED),
			},
		}

		status, err := serviceOver(t, queued).Status(context.Background(), "RefreshMetadata")
		if err != nil {
			t.Fatalf("Status returned %v", err)
		}
		if status.State != StateIdle {
			t.Errorf("state = %q, want %q with nothing running", status.State, StateIdle)
		}
		if status.Last == nil {
			t.Fatal("no last result, want the completed run")
		}
		if !status.Last.Succeeded {
			t.Error("the last result did not succeed, want a completed run")
		}
	})
}

func TestService_Cancel(t *testing.T) {
	scoped := Params{"scope": "b3f"}.id("RefreshMetadata")
	queued := &queue{
		running: []*workflow.WorkflowExecutionInfo{
			execution(scoped, enums.WORKFLOW_EXECUTION_STATUS_RUNNING),
		},
	}

	if err := serviceOver(t, queued).Cancel(context.Background(), "RefreshMetadata"); err != nil {
		t.Fatalf("Cancel returned %v", err)
	}
	if len(queued.cancelled) != 1 || queued.cancelled[0] != scoped {
		t.Errorf("cancelled = %v, want only %q", queued.cancelled, scoped)
	}
}
