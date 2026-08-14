package scheduledtasks

import (
	"github.com/FreekingDean/gojellyfin/internal/jobs"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

func taskInfo(status jobs.Status) api.TaskInfo {
	triggers := make([]api.TaskTriggerInfo, 0)

	return api.TaskInfo{
		Id:                  apiutil.Ptr(status.Job.Name()),
		Key:                 apiutil.Ptr(status.Job.Name()),
		Name:                apiutil.Ptr(status.Job.Name()),
		Description:         apiutil.Ptr(status.Job.Description()),
		Category:            apiutil.Ptr(status.Job.Category()),
		IsHidden:            apiutil.Ptr(false),
		State:               apiutil.Ptr(taskState(status.State)),
		Triggers:            &triggers,
		LastExecutionResult: taskResult(status),
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
		Id:           apiutil.Ptr(status.Job.Name()),
		Key:          apiutil.Ptr(status.Job.Name()),
		Name:         apiutil.Ptr(status.Job.Name()),
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
