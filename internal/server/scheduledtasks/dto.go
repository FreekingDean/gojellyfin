package scheduledtasks

import (
	"slices"
	"time"

	"github.com/FreekingDean/gojellyfin/internal/jobs"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

func taskInfo(status jobs.Status, scheduled []jobs.Trigger) api.TaskInfo {
	triggers := make([]api.TaskTriggerInfo, 0, len(scheduled))
	for _, trigger := range scheduled {
		triggers = append(triggers, triggerInfo(trigger))
	}

	return api.TaskInfo{
		Id:                  apiutil.Ptr(status.Job.Name),
		Key:                 apiutil.Ptr(status.Job.Name),
		Name:                apiutil.Ptr(status.Job.Name),
		Description:         apiutil.Ptr(status.Job.Description),
		Category:            apiutil.Ptr(status.Job.Category),
		IsHidden:            apiutil.Ptr(false),
		State:               apiutil.Ptr(taskState(status.State)),
		Triggers:            &triggers,
		LastExecutionResult: taskResult(status),

		CurrentProgressPercentage: status.Progress,
	}
}

func taskState(state jobs.State) api.TaskState {
	switch state {
	case jobs.StateRunning:
		return api.TaskStateRunning
	case jobs.StateCancelling:
		return api.TaskStateCancelling
	default:
		return api.TaskStateIdle
	}
}

func taskResult(status jobs.Status) *api.TaskResult {
	if status.Last == nil {
		return nil
	}

	return &api.TaskResult{
		Id:           apiutil.Ptr(status.Job.Name),
		Key:          apiutil.Ptr(status.Job.Name),
		Name:         apiutil.Ptr(status.Job.Name),
		StartTimeUtc: apiutil.Ptr(status.Last.StartedAt.UTC()),
		EndTimeUtc:   apiutil.Ptr(status.Last.EndedAt.UTC()),
		Status:       apiutil.Ptr(completionStatus(status.Last)),
	}
}

func completionStatus(result *jobs.Result) api.TaskCompletionStatus {
	switch {
	case result.Succeeded:
		return api.TaskCompletionStatusCompleted
	case result.Cancelled:
		return api.TaskCompletionStatusCancelled
	default:
		return api.TaskCompletionStatusFailed
	}
}

const tick = 100 * time.Nanosecond

var triggerKinds = map[api.TaskTriggerInfoType]jobs.TriggerKind{
	api.DailyTrigger:    jobs.TriggerDaily,
	api.WeeklyTrigger:   jobs.TriggerWeekly,
	api.IntervalTrigger: jobs.TriggerInterval,
	api.StartupTrigger:  jobs.TriggerStartup,
}

var weekdays = []api.DayOfWeek{
	api.DayOfWeekSunday,
	api.DayOfWeekMonday,
	api.DayOfWeekTuesday,
	api.DayOfWeekWednesday,
	api.DayOfWeekThursday,
	api.DayOfWeekFriday,
	api.DayOfWeekSaturday,
}

func ticks(duration time.Duration) *int64 {
	return apiutil.Ptr(int64(duration / tick))
}

func duration(ticks *int64) time.Duration {
	return time.Duration(apiutil.Deref(ticks)) * tick
}

func triggerInfo(trigger jobs.Trigger) api.TaskTriggerInfo {
	info := api.TaskTriggerInfo{}
	for kind, ours := range triggerKinds {
		if ours == trigger.Kind {
			info.Type = apiutil.Ptr(kind)
		}
	}

	switch trigger.Kind {
	case jobs.TriggerInterval:
		info.IntervalTicks = ticks(trigger.Interval)
	case jobs.TriggerWeekly:
		info.DayOfWeek = apiutil.Ptr(weekdays[trigger.Day])
		info.TimeOfDayTicks = ticks(trigger.TimeOfDay)
	case jobs.TriggerDaily:
		info.TimeOfDayTicks = ticks(trigger.TimeOfDay)
	case jobs.TriggerStartup:
	}
	if trigger.MaxRuntime > 0 {
		info.MaxRuntimeTicks = ticks(trigger.MaxRuntime)
	}

	return info
}

func trigger(info api.TaskTriggerInfo) jobs.Trigger {
	return jobs.Trigger{
		Kind:       triggerKinds[apiutil.Deref(info.Type)],
		TimeOfDay:  duration(info.TimeOfDayTicks),
		Interval:   duration(info.IntervalTicks),
		Day:        time.Weekday(slices.Index(weekdays, apiutil.OrElse(info.DayOfWeek, api.DayOfWeekSunday))),
		MaxRuntime: duration(info.MaxRuntimeTicks),
	}
}
