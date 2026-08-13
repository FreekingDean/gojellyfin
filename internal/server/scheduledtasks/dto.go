package scheduledtasks

import (
	"github.com/FreekingDean/gojellyfin/internal/jobs"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

func taskInfo(summary jobs.Summary) api.TaskInfo {
	converted := make([]api.TaskTriggerInfo, 0, len(summary.Triggers))
	for _, trigger := range summary.Triggers {
		converted = append(converted, api.TaskTriggerInfo{
			Type:            apiutil.Ptr(api.TaskTriggerInfoType(trigger.Type)),
			IntervalTicks:   trigger.IntervalTicks,
			TimeOfDayTicks:  trigger.TimeOfDayTicks,
			MaxRuntimeTicks: trigger.MaxRuntimeTicks,
			DayOfWeek:       dayOfWeek(trigger.DayOfWeek),
		})
	}

	return api.TaskInfo{
		Id:                        apiutil.Ptr(summary.Definition.Kind),
		Key:                       apiutil.Ptr(summary.Definition.Kind),
		Name:                      apiutil.Ptr(summary.Definition.Name),
		Description:               apiutil.Ptr(summary.Definition.Description),
		Category:                  apiutil.Ptr(summary.Definition.Category),
		IsHidden:                  apiutil.Ptr(false),
		State:                     apiutil.Ptr(taskState(summary.Active)),
		CurrentProgressPercentage: progress(summary.Active),
		Triggers:                  &converted,
		LastExecutionResult:       taskResult(summary),
	}
}

func triggers(infos []api.TaskTriggerInfo) []jobs.Trigger {
	converted := make([]jobs.Trigger, 0, len(infos))
	for _, info := range infos {
		trigger := jobs.Trigger{
			Type:            string(apiutil.Deref(info.Type)),
			IntervalTicks:   info.IntervalTicks,
			TimeOfDayTicks:  info.TimeOfDayTicks,
			MaxRuntimeTicks: info.MaxRuntimeTicks,
		}
		if info.DayOfWeek != nil {
			trigger.DayOfWeek = apiutil.Ptr(string(*info.DayOfWeek))
		}
		converted = append(converted, trigger)
	}

	return converted
}

func taskState(active *jobs.Job) api.TaskState {
	switch {
	case active == nil:
		return api.TaskStateIdle
	case active.CancelRequested:
		return api.TaskStateCancelling
	default:
		return api.TaskStateRunning
	}
}

func progress(active *jobs.Job) *float64 {
	if active == nil || active.State != jobs.StateRunning {
		return nil
	}

	return apiutil.Ptr(active.Progress)
}

func taskResult(summary jobs.Summary) *api.TaskResult {
	if summary.Last == nil {
		return nil
	}

	result := &api.TaskResult{
		Id:           apiutil.Ptr(summary.Definition.Kind),
		Key:          apiutil.Ptr(summary.Definition.Kind),
		Name:         apiutil.Ptr(summary.Definition.Name),
		StartTimeUtc: apiutil.Ptr(summary.Last.StartedAt.UTC()),
		EndTimeUtc:   apiutil.Ptr(summary.Last.FinishedAt.UTC()),
		Status:       apiutil.Ptr(completionStatus(summary.Last.State)),
	}
	if summary.Last.ErrorMessage != "" {
		result.ErrorMessage = apiutil.Ptr(summary.Last.ErrorMessage)
	}

	return result
}

func completionStatus(state jobs.State) api.TaskCompletionStatus {
	switch state {
	case jobs.StateSucceeded:
		return api.TaskCompletionStatusCompleted
	case jobs.StateCancelled:
		return api.TaskCompletionStatusCancelled
	default:
		return api.TaskCompletionStatusFailed
	}
}

func dayOfWeek(day *string) *api.DayOfWeek {
	if day == nil {
		return nil
	}

	return apiutil.Ptr(api.DayOfWeek(*day))
}
