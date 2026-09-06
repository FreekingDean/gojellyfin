package scheduledtasks

import (
	"context"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/jobs"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

func newServer(t *testing.T) *Server {
	t.Helper()

	client, err := jobs.NewClient(env.Config{})
	if err != nil {
		t.Fatalf("failed to build the temporal client: %v", err)
	}

	registry := jobs.NewRegistry()
	registry.Register(scanJob{})

	return New(jobs.NewService(client, registry))
}

func TestServer_GetTasks(t *testing.T) {
	t.Run("lists the definitions", func(t *testing.T) {
		response, err := newServer(t).GetTasks(context.Background(), api.GetTasksRequestObject{})
		if err != nil {
			t.Fatalf("GetTasks returned %v", err)
		}

		infos, ok := response.(api.GetTasks200JSONResponse)
		if !ok {
			t.Fatalf("response = %T, want api.GetTasks200JSONResponse", response)
		}
		if len(infos) == 0 {
			t.Fatal("no tasks were listed")
		}
		if got := apiutil.Deref(infos[0].Id); got != (scanJob{}).Name() {
			t.Errorf("id = %q, want %q", got, (scanJob{}).Name())
		}
		if got := apiutil.Deref(infos[0].State); got != api.TaskStateIdle {
			t.Errorf("state = %q, want Idle with nothing running", got)
		}
	})

	t.Run("filters", func(t *testing.T) {
		server := newServer(t)
		ctx := context.Background()

		for _, params := range []api.GetTasksParams{
			{IsHidden: apiutil.Ptr(true)},
			{IsEnabled: apiutil.Ptr(false)},
		} {
			response, err := server.GetTasks(ctx, api.GetTasksRequestObject{Params: params})
			if err != nil {
				t.Fatalf("GetTasks returned %v", err)
			}
			if infos, _ := response.(api.GetTasks200JSONResponse); len(infos) != 0 {
				t.Errorf("params %+v listed %d tasks, want none", params, len(infos))
			}
		}
	})
}

func TestServer(t *testing.T) {
	server := newServer(t)
	ctx := context.Background()

	get, err := server.GetTask(ctx, api.GetTaskRequestObject{TaskId: "NoSuchTask"})
	if err != nil {
		t.Fatalf("GetTask returned %v", err)
	}
	if _, ok := get.(api.GetTask404JSONResponse); !ok {
		t.Errorf("GetTask = %T, want 404", get)
	}

	start, err := server.StartTask(ctx, api.StartTaskRequestObject{TaskId: "NoSuchTask"})
	if err != nil {
		t.Fatalf("StartTask returned %v", err)
	}
	if _, ok := start.(api.StartTask404JSONResponse); !ok {
		t.Errorf("StartTask = %T, want 404", start)
	}

	stop, err := server.StopTask(ctx, api.StopTaskRequestObject{TaskId: "NoSuchTask"})
	if err != nil {
		t.Fatalf("StopTask returned %v", err)
	}
	if _, ok := stop.(api.StopTask404JSONResponse); !ok {
		t.Errorf("StopTask = %T, want 404", stop)
	}
}

type scanJob struct{}

func (scanJob) Name() string        { return "RefreshLibrary" }
func (scanJob) Description() string { return "Scans the media libraries." }
func (scanJob) Category() string    { return "Library" }
func (scanJob) Queue() string       { return "gojellyfin" }
func (scanJob) Steps() []any        { return nil }
func (scanJob) Children() []any     { return nil }
func (scanJob) Run(jobs.Context, jobs.Options) error {
	return nil
}
