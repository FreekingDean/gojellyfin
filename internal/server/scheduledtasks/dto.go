package scheduledtasks

import (
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/tasks"
)

func taskInfo(info tasks.Info) api.TaskInfo {
	converted := make([]api.TaskTriggerInfo, 0, len(info.Triggers))
	for _, trigger := range info.Triggers {
		converted = append(converted, api.TaskTriggerInfo{
			Type:            apiutil.Ptr(api.TaskTriggerInfoType(trigger.Type)),
			IntervalTicks:   trigger.IntervalTicks,
			TimeOfDayTicks:  trigger.TimeOfDayTicks,
			MaxRuntimeTicks: trigger.MaxRuntimeTicks,
			DayOfWeek:       dayOfWeek(trigger.DayOfWeek),
		})
	}

	return api.TaskInfo{
		Id:                  apiutil.Ptr(info.ID),
		Key:                 apiutil.Ptr(info.ID),
		Name:                apiutil.Ptr(info.Name),
		Description:         apiutil.Ptr(info.Description),
		Category:            apiutil.Ptr(info.Category),
		IsHidden:            apiutil.Ptr(false),
		State:               apiutil.Ptr(api.TaskState(info.State)),
		Triggers:            &converted,
		LastExecutionResult: taskResult(info),
	}
}

func triggers(infos []api.TaskTriggerInfo) []tasks.Trigger {
	converted := make([]tasks.Trigger, 0, len(infos))
	for _, info := range infos {
		trigger := tasks.Trigger{
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

func taskResult(info tasks.Info) *api.TaskResult {
	if info.LastResult == nil {
		return nil
	}

	result := &api.TaskResult{
		Id:           apiutil.Ptr(info.ID),
		Key:          apiutil.Ptr(info.ID),
		Name:         apiutil.Ptr(info.Name),
		StartTimeUtc: apiutil.Ptr(info.LastResult.StartedAt.UTC()),
		EndTimeUtc:   apiutil.Ptr(info.LastResult.EndedAt.UTC()),
		Status:       apiutil.Ptr(api.TaskCompletionStatus(info.LastResult.Status)),
	}
	if info.LastResult.Error != "" {
		result.ErrorMessage = apiutil.Ptr(info.LastResult.Error)
	}

	return result
}

func dayOfWeek(day *string) *api.DayOfWeek {
	if day == nil {
		return nil
	}

	return apiutil.Ptr(api.DayOfWeek(*day))
}
