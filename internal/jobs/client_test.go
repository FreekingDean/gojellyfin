package jobs

import (
	"context"
	"errors"
	"testing"

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

type queuedJob struct {
	name  string
	queue string
}

func (q queuedJob) Name() string                   { return q.name }
func (q queuedJob) Queue() string                  { return q.queue }
func (q queuedJob) Category() string               { return "Library" }
func (q queuedJob) Description() string            { return "" }
func (q queuedJob) Steps() []any                   { return nil }
func (q queuedJob) Children() []any                { return nil }
func (q queuedJob) Run(_ Context, _ Options) error { return nil }

type recordingClient struct {
	client.Client
	started client.StartWorkflowOptions
}

func (r *recordingClient) ExecuteWorkflow(
	_ context.Context,
	options client.StartWorkflowOptions,
	_ any,
	_ ...any,
) (client.WorkflowRun, error) {
	r.started = options

	return nil, nil
}

func TestStartNamesTheJobsQueue(t *testing.T) {
	recorder := &recordingClient{}
	registry := NewRegistry()
	registry.Register(queuedJob{name: "RefreshMetadata", queue: "gojellyfin-metadata"})

	err := NewService(&Client{client: recorder}, registry).
		Start(context.Background(), "RefreshMetadata", Options{})
	if err != nil {
		t.Fatalf("Start returned %v", err)
	}

	if recorder.started.TaskQueue != "gojellyfin-metadata" {
		t.Errorf("TaskQueue = %q, want gojellyfin-metadata", recorder.started.TaskQueue)
	}
	if recorder.started.ID != "RefreshMetadata" {
		t.Errorf("ID = %q, want RefreshMetadata", recorder.started.ID)
	}
}
