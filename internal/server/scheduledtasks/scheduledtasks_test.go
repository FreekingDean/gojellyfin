package scheduledtasks

import (
	"context"
	"testing"
	"time"

	"github.com/FreekingDean/gojellyfin/internal/jobs"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/store"
	jobmodal "github.com/FreekingDean/gojellyfin/internal/store/job"
	schedulemodal "github.com/FreekingDean/gojellyfin/internal/store/jobschedule"
)

func TestGetTasks(t *testing.T) {
	server, _, _ := testServer(t)

	response, err := server.GetTasks(context.Background(), api.GetTasksRequestObject{})
	if err != nil {
		t.Fatal(err)
	}

	infos := response.(api.GetTasks200JSONResponse)
	if len(infos) != 1 {
		t.Fatalf("got %d tasks, want 1", len(infos))
	}
	if got := apiutil.Deref(infos[0].Id); got != jobs.LibraryScanKind {
		t.Errorf("got id %q, want %q", got, jobs.LibraryScanKind)
	}
	if got := apiutil.Deref(infos[0].State); got != api.TaskStateIdle {
		t.Errorf("got state %q, want %q", got, api.TaskStateIdle)
	}
	if infos[0].LastExecutionResult != nil {
		t.Errorf("got a last result for a task that never ran: %+v", infos[0].LastExecutionResult)
	}
}

func TestGetTasksFiltered(t *testing.T) {
	server, _, _ := testServer(t)

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
	server, _, _ := testServer(t)

	response, err := server.GetTask(context.Background(), api.GetTaskRequestObject{TaskId: "nope"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := response.(api.GetTask404JSONResponse); !ok {
		t.Fatalf("got %T, want a 404", response)
	}
}

func TestStartTaskQueuesOneJob(t *testing.T) {
	server, _, client := testServer(t)
	ctx := context.Background()

	for range 2 {
		response, err := server.StartTask(ctx, api.StartTaskRequestObject{TaskId: jobs.LibraryScanKind})
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := response.(api.StartTask204Response); !ok {
			t.Fatalf("got %T, want a 204", response)
		}
	}

	if got := apiutil.Deref(fetchTaskInfo(t, server, jobs.LibraryScanKind).State); got != api.TaskStateRunning {
		t.Errorf("got state %q, want %q", got, api.TaskStateRunning)
	}
	count, err := client.Job.Query().Where(jobmodal.Kind(jobs.LibraryScanKind)).Count(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("got %d jobs, want the second start deduplicated", count)
	}
}

func TestStopTaskCancelsTheQueuedJob(t *testing.T) {
	server, _, _ := testServer(t)
	ctx := context.Background()

	if _, err := server.StartTask(ctx, api.StartTaskRequestObject{TaskId: jobs.LibraryScanKind}); err != nil {
		t.Fatal(err)
	}

	response, err := server.StopTask(ctx, api.StopTaskRequestObject{TaskId: jobs.LibraryScanKind})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := response.(api.StopTask204Response); !ok {
		t.Fatalf("got %T, want a 204", response)
	}

	info := fetchTaskInfo(t, server, jobs.LibraryScanKind)
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

func TestStopTaskReportsCancellingWhileAWorkerHoldsTheJob(t *testing.T) {
	server, service, _ := testServer(t)
	ctx := context.Background()

	if _, err := server.StartTask(ctx, api.StartTaskRequestObject{TaskId: jobs.LibraryScanKind}); err != nil {
		t.Fatal(err)
	}
	held, err := service.Lease(ctx, "one", []string{jobs.LibraryScanKind}, time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := server.StopTask(ctx, api.StopTaskRequestObject{TaskId: jobs.LibraryScanKind}); err != nil {
		t.Fatal(err)
	}
	if got := apiutil.Deref(fetchTaskInfo(t, server, jobs.LibraryScanKind).State); got != api.TaskStateCancelling {
		t.Errorf("got state %q, want %q", got, api.TaskStateCancelling)
	}

	requested, err := service.Renew(ctx, held, time.Minute, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !requested {
		t.Fatal("the holder never saw the cancellation")
	}
	if err := service.Cancelled(ctx, held); err != nil {
		t.Fatal(err)
	}

	if got := apiutil.Deref(fetchTaskInfo(t, server, jobs.LibraryScanKind).State); got != api.TaskStateIdle {
		t.Errorf("got state %q, want %q", got, api.TaskStateIdle)
	}
}

func TestStopTaskWithNothingRunning(t *testing.T) {
	server, _, _ := testServer(t)

	response, err := server.StopTask(context.Background(), api.StopTaskRequestObject{TaskId: jobs.LibraryScanKind})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := response.(api.StopTask204Response); !ok {
		t.Fatalf("got %T, want a 204", response)
	}
}

func TestTaskReportsTheLastRunAndItsProgress(t *testing.T) {
	server, service, _ := testServer(t)
	ctx := context.Background()

	if _, err := server.StartTask(ctx, api.StartTaskRequestObject{TaskId: jobs.LibraryScanKind}); err != nil {
		t.Fatal(err)
	}
	held, err := service.Lease(ctx, "one", []string{jobs.LibraryScanKind}, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Renew(ctx, held, time.Minute, 40); err != nil {
		t.Fatal(err)
	}

	if got := apiutil.Deref(fetchTaskInfo(t, server, jobs.LibraryScanKind).CurrentProgressPercentage); got != 40 {
		t.Errorf("got progress %v, want 40", got)
	}

	if err := service.Succeed(ctx, held); err != nil {
		t.Fatal(err)
	}

	info := fetchTaskInfo(t, server, jobs.LibraryScanKind)
	if info.LastExecutionResult == nil {
		t.Fatal("got no last execution result")
	}
	if got := apiutil.Deref(info.LastExecutionResult.Status); got != api.TaskCompletionStatusCompleted {
		t.Errorf("got status %q, want %q", got, api.TaskCompletionStatusCompleted)
	}
	if apiutil.Deref(info.LastExecutionResult.EndTimeUtc).Before(apiutil.Deref(info.LastExecutionResult.StartTimeUtc)) {
		t.Errorf("got %+v, want it to end after it started", info.LastExecutionResult)
	}
	if info.CurrentProgressPercentage != nil {
		t.Errorf("got progress %v for an idle task, want none", *info.CurrentProgressPercentage)
	}
}

func TestUpdateTask(t *testing.T) {
	server, _, _ := testServer(t)
	body := api.UpdateTaskJSONRequestBody{{
		Type:            apiutil.Ptr(api.TaskTriggerInfoType("IntervalTrigger")),
		IntervalTicks:   apiutil.Ptr(int64(36000000000)),
		MaxRuntimeTicks: apiutil.Ptr(int64(3600000000)),
		DayOfWeek:       apiutil.Ptr(api.DayOfWeekSunday),
	}}

	response, err := server.UpdateTask(context.Background(), api.UpdateTaskRequestObject{TaskId: jobs.LibraryScanKind, JSONBody: &body})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := response.(api.UpdateTask204Response); !ok {
		t.Fatalf("got %T, want a 204", response)
	}

	triggers := apiutil.Deref(fetchTaskInfo(t, server, jobs.LibraryScanKind).Triggers)
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
	server, service, _ := testServer(t)
	ctx := context.Background()
	if err := service.SetTriggers(ctx, jobs.LibraryScanKind, []jobs.Trigger{{Type: "IntervalTrigger"}}); err != nil {
		t.Fatal(err)
	}

	response, err := server.UpdateTask(ctx, api.UpdateTaskRequestObject{TaskId: jobs.LibraryScanKind})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := response.(api.UpdateTask204Response); !ok {
		t.Fatalf("got %T, want a 204", response)
	}
	if triggers := apiutil.Deref(fetchTaskInfo(t, server, jobs.LibraryScanKind).Triggers); len(triggers) != 0 {
		t.Errorf("got %d triggers, want none", len(triggers))
	}
}

func TestUpdateUnknownTask(t *testing.T) {
	server, _, _ := testServer(t)
	body := api.UpdateTaskJSONRequestBody{}

	response, err := server.UpdateTask(context.Background(), api.UpdateTaskRequestObject{TaskId: "nope", JSONBody: &body})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := response.(api.UpdateTask404JSONResponse); !ok {
		t.Fatalf("got %T, want a 404", response)
	}
}

func TestStartUnknownTask(t *testing.T) {
	server, _, _ := testServer(t)

	response, err := server.StartTask(context.Background(), api.StartTaskRequestObject{TaskId: "nope"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := response.(api.StartTask404JSONResponse); !ok {
		t.Errorf("got %T, want a 404", response)
	}

	stopped, err := server.StopTask(context.Background(), api.StopTaskRequestObject{TaskId: "nope"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := stopped.(api.StopTask404JSONResponse); !ok {
		t.Errorf("got %T, want a 404", stopped)
	}
}

func testServer(t *testing.T) (*Server, *jobs.Service, *store.Client) {
	t.Helper()

	connection, err := store.NewStore()
	if err != nil {
		t.Fatalf("failed to open the database: %v", err)
	}
	if err := connection.Start(); err != nil {
		t.Fatalf("failed to reach the database, set DATABASE_URL: %v", err)
	}

	client := connection.Client()
	clear := func() {
		ctx := context.Background()
		if _, err := client.Job.Delete().Where(jobmodal.Kind(jobs.LibraryScanKind)).Exec(ctx); err != nil {
			t.Fatalf("failed to delete the jobs: %v", err)
		}
		if _, err := client.JobSchedule.Delete().Where(schedulemodal.Kind(jobs.LibraryScanKind)).Exec(ctx); err != nil {
			t.Fatalf("failed to delete the schedule: %v", err)
		}
	}

	clear()
	t.Cleanup(func() {
		clear()
		if err := connection.Stop(); err != nil {
			t.Errorf("failed to close the database: %v", err)
		}
	})

	service := jobs.New(client)

	return New(service), service, client
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
