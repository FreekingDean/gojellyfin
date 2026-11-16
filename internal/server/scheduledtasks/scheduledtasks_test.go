package scheduledtasks

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/jobs"
	"github.com/FreekingDean/gojellyfin/internal/observability/tracing"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

func newServer(t *testing.T) *Server {
	t.Helper()

	client, err := jobs.NewClient(env.Config{}, &tracing.Tracing{})
	if err != nil {
		t.Fatalf("failed to build the temporal client: %v", err)
	}

	registry := jobs.NewRegistry()
	registry.Register(scanJob)

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
		if got := apiutil.Deref(infos[0].Id); got != scanJob.Name {
			t.Errorf("id = %q, want %q", got, scanJob.Name)
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

var scanJob = jobs.Job{
	Name:        "RefreshLibrary",
	Description: "Scans the media libraries.",
	Category:    "Library",
	Startable:   true,
	Run:         func(context.Context) error { return nil },
}

func TestTrigger(t *testing.T) {
	hour := int64(36_000_000_000)
	for name, info := range map[string]api.TaskTriggerInfo{
		"daily":    {Type: apiutil.Ptr(api.DailyTrigger), TimeOfDayTicks: apiutil.Ptr(3 * hour)},
		"weekly":   {Type: apiutil.Ptr(api.WeeklyTrigger), TimeOfDayTicks: apiutil.Ptr(22 * hour), DayOfWeek: apiutil.Ptr(api.DayOfWeekFriday)},
		"interval": {Type: apiutil.Ptr(api.IntervalTrigger), IntervalTicks: apiutil.Ptr(12 * hour), MaxRuntimeTicks: apiutil.Ptr(2 * hour)},
		"startup":  {Type: apiutil.Ptr(api.StartupTrigger)},
	} {
		t.Run(name, func(t *testing.T) {
			got := triggerInfo(trigger(info))
			if !reflect.DeepEqual(got, info) {
				t.Errorf("round trip = %s, want %s", describe(got), describe(info))
			}
		})
	}

	t.Run("reads ticks as time", func(t *testing.T) {
		got := trigger(api.TaskTriggerInfo{
			Type:           apiutil.Ptr(api.WeeklyTrigger),
			TimeOfDayTicks: apiutil.Ptr(3*hour + hour/2),
			DayOfWeek:      apiutil.Ptr(api.DayOfWeekMonday),
		})
		want := jobs.Trigger{Kind: jobs.TriggerWeekly, TimeOfDay: 3*time.Hour + 30*time.Minute, Day: time.Monday}
		if got != want {
			t.Errorf("trigger = %+v, want %+v", got, want)
		}
	})
}

func describe(info api.TaskTriggerInfo) string {
	written, _ := json.Marshal(info)

	return string(written)
}

func TestServer_UpdateTask(t *testing.T) {
	response, err := newServer(t).UpdateTask(context.Background(), api.UpdateTaskRequestObject{
		TaskId:   "NoSuchTask",
		JSONBody: &api.UpdateTaskJSONRequestBody{},
	})
	if err != nil {
		t.Fatalf("UpdateTask returned %v", err)
	}
	if _, ok := response.(api.UpdateTask404JSONResponse); !ok {
		t.Errorf("UpdateTask = %T, want 404", response)
	}
}
