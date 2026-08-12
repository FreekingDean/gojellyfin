package scheduledtasks

import (
	"context"
	"testing"
	"time"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/tasks"
)

func TestGetTasks(t *testing.T) {
	server, _ := testServer(t, func(context.Context) error { return nil })

	response, err := server.GetTasks(context.Background(), api.GetTasksRequestObject{})
	if err != nil {
		t.Fatal(err)
	}

	infos := response.(api.GetTasks200JSONResponse)
	if len(infos) != 1 {
		t.Fatalf("got %d tasks, want 1", len(infos))
	}
	if got := apiutil.Deref(infos[0].Id); got != tasks.LibraryScanID {
		t.Errorf("got id %q, want %q", got, tasks.LibraryScanID)
	}
	if got := apiutil.Deref(infos[0].State); got != api.TaskStateIdle {
		t.Errorf("got state %q, want %q", got, api.TaskStateIdle)
	}
	if infos[0].LastExecutionResult != nil {
		t.Errorf("got a last result for a task that never ran: %+v", infos[0].LastExecutionResult)
	}
}

func TestGetTasksFiltered(t *testing.T) {
	server, _ := testServer(t, func(context.Context) error { return nil })

	for _, params := range []api.GetTasksParams{
		{IsHidden: apiutil.Ptr(true)},
		{IsEnabled: apiutil.Ptr(false)},
	} {
		response, err := server.GetTasks(context.Background(), api.GetTasksRequestObject{Params: params})
		if err != nil {
			t.Fatal(err)
		}
		if infos := response.(api.GetTasks200JSONResponse); len(infos) != 0 {
			t.Errorf("%+v: got %d tasks, want 0", params, len(infos))
		}
	}
}

func TestGetTaskUnknown(t *testing.T) {
	server, _ := testServer(t, func(context.Context) error { return nil })

	response, err := server.GetTask(context.Background(), api.GetTaskRequestObject{TaskId: "nope"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := response.(api.GetTask404JSONResponse); !ok {
		t.Fatalf("got %T, want a 404", response)
	}
}

func TestStartAndStopTask(t *testing.T) {
	cancelled := make(chan struct{})
	server, registry := testServer(t, func(ctx context.Context) error {
		<-ctx.Done()
		close(cancelled)
		return ctx.Err()
	})

	started, err := server.StartTask(context.Background(), api.StartTaskRequestObject{TaskId: tasks.LibraryScanID})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := started.(api.StartTask204Response); !ok {
		t.Fatalf("got %T, want a 204", started)
	}
	if got := apiutil.Deref(fetchTaskInfo(t, server, tasks.LibraryScanID).State); got != api.TaskStateRunning {
		t.Fatalf("got state %q, want %q", got, api.TaskStateRunning)
	}

	stopped, err := server.StopTask(context.Background(), api.StopTaskRequestObject{TaskId: tasks.LibraryScanID})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := stopped.(api.StopTask204Response); !ok {
		t.Fatalf("got %T, want a 204", stopped)
	}

	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("the task was never cancelled")
	}
	waitForIdle(t, registry)

	info := fetchTaskInfo(t, server, tasks.LibraryScanID)
	if got := apiutil.Deref(info.State); got != api.TaskStateIdle {
		t.Errorf("got state %q, want %q", got, api.TaskStateIdle)
	}
	if info.LastExecutionResult == nil {
		t.Fatal("got no last execution result")
	}
	if got := apiutil.Deref(info.LastExecutionResult.Status); got != api.TaskCompletionStatusCancelled {
		t.Errorf("got status %q, want %q", got, api.TaskCompletionStatusCancelled)
	}
}

func TestStartAndStopUnknownTask(t *testing.T) {
	server, _ := testServer(t, func(context.Context) error { return nil })

	started, err := server.StartTask(context.Background(), api.StartTaskRequestObject{TaskId: "nope"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := started.(api.StartTask404JSONResponse); !ok {
		t.Errorf("got %T, want a 404", started)
	}

	stopped, err := server.StopTask(context.Background(), api.StopTaskRequestObject{TaskId: "nope"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := stopped.(api.StopTask404JSONResponse); !ok {
		t.Errorf("got %T, want a 404", stopped)
	}
}

func TestUpdateTask(t *testing.T) {
	server, _ := testServer(t, func(context.Context) error { return nil })
	body := api.UpdateTaskJSONRequestBody{{
		Type:            apiutil.Ptr("IntervalTrigger"),
		IntervalTicks:   apiutil.Ptr(int64(36000000000)),
		MaxRuntimeTicks: apiutil.Ptr(int64(3600000000)),
		DayOfWeek:       apiutil.Ptr(api.DayOfWeekSunday),
	}}

	response, err := server.UpdateTask(context.Background(), api.UpdateTaskRequestObject{TaskId: tasks.LibraryScanID, JSONBody: &body})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := response.(api.UpdateTask204Response); !ok {
		t.Fatalf("got %T, want a 204", response)
	}

	triggers := apiutil.Deref(fetchTaskInfo(t, server, tasks.LibraryScanID).Triggers)
	if len(triggers) != 1 {
		t.Fatalf("got %d triggers, want 1", len(triggers))
	}
	if got := apiutil.Deref(triggers[0].Type); got != "IntervalTrigger" {
		t.Errorf("got type %q, want %q", got, "IntervalTrigger")
	}
	if got := apiutil.Deref(triggers[0].IntervalTicks); got != 36000000000 {
		t.Errorf("got interval %d, want %d", got, 36000000000)
	}
	if got := apiutil.Deref(triggers[0].DayOfWeek); got != api.DayOfWeekSunday {
		t.Errorf("got day %q, want %q", got, api.DayOfWeekSunday)
	}
}

func TestUpdateTaskWithoutBody(t *testing.T) {
	server, registry := testServer(t, func(context.Context) error { return nil })
	if err := registry.SetTriggers(tasks.LibraryScanID, []tasks.Trigger{{Type: "IntervalTrigger"}}); err != nil {
		t.Fatal(err)
	}

	response, err := server.UpdateTask(context.Background(), api.UpdateTaskRequestObject{TaskId: tasks.LibraryScanID})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := response.(api.UpdateTask204Response); !ok {
		t.Fatalf("got %T, want a 204", response)
	}
	if triggers := apiutil.Deref(fetchTaskInfo(t, server, tasks.LibraryScanID).Triggers); len(triggers) != 0 {
		t.Errorf("got %d triggers, want none", len(triggers))
	}
}

func TestUpdateUnknownTask(t *testing.T) {
	server, _ := testServer(t, func(context.Context) error { return nil })
	body := api.UpdateTaskJSONRequestBody{}

	response, err := server.UpdateTask(context.Background(), api.UpdateTaskRequestObject{TaskId: "nope", JSONBody: &body})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := response.(api.UpdateTask404JSONResponse); !ok {
		t.Fatalf("got %T, want a 404", response)
	}
}

func testServer(t *testing.T, run tasks.Runner) (*Server, *tasks.Registry) {
	t.Helper()

	registry := tasks.New()
	if err := registry.UseRunner(tasks.LibraryScanID, run); err != nil {
		t.Fatal(err)
	}

	return New(registry), registry
}

func fetchTaskInfo(t *testing.T, server *Server, id string) api.TaskInfo {
	t.Helper()

	response, err := server.GetTask(context.Background(), api.GetTaskRequestObject{TaskId: id})
	if err != nil {
		t.Fatal(err)
	}
	found, ok := response.(api.GetTask200JSONResponse)
	if !ok {
		t.Fatalf("got %T, want a 200", response)
	}

	return api.TaskInfo(found)
}

func waitForIdle(t *testing.T, registry *tasks.Registry) {
	t.Helper()

	for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); {
		found, err := registry.Task(tasks.LibraryScanID)
		if err != nil {
			t.Fatal(err)
		}
		if found.State == tasks.StateIdle && found.LastResult != nil {
			return
		}
		time.Sleep(time.Millisecond)
	}

	t.Fatalf("task %q never went idle", tasks.LibraryScanID)
}
