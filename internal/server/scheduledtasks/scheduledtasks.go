package scheduledtasks

import (
	"context"
	"errors"

	"github.com/FreekingDean/gojellyfin/internal/jobs"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

type Server struct {
	jobs *jobs.Service
}

func New(service *jobs.Service) *Server {
	return &Server{jobs: service}
}

func (s *Server) GetTasks(ctx context.Context, request api.GetTasksRequestObject) (api.GetTasksResponseObject, error) {
	hiddenOnly := apiutil.Deref(request.Params.IsHidden)
	disabledOnly := !apiutil.OrElse(request.Params.IsEnabled, true)

	infos := make([]api.TaskInfo, 0)
	if hiddenOnly || disabledOnly {
		return api.GetTasks200JSONResponse(infos), nil
	}

	statuses, err := s.jobs.All(ctx)
	if err != nil {
		return nil, err
	}
	for _, status := range statuses {
		triggers, err := s.jobs.Triggers(ctx, status.Job.Name)
		if err != nil {
			return nil, err
		}
		infos = append(infos, taskInfo(status, triggers))
	}

	return api.GetTasks200JSONResponse(infos), nil
}

func (s *Server) GetTask(ctx context.Context, request api.GetTaskRequestObject) (api.GetTaskResponseObject, error) {
	status, err := s.jobs.Status(ctx, request.TaskId)
	if errors.Is(err, jobs.ErrNotFound) {
		return api.GetTask404JSONResponse{}, nil
	}
	if err != nil {
		return nil, err
	}

	triggers, err := s.jobs.Triggers(ctx, request.TaskId)
	if err != nil {
		return nil, err
	}

	return api.GetTask200JSONResponse(taskInfo(status, triggers)), nil
}

func (s *Server) StartTask(ctx context.Context, request api.StartTaskRequestObject) (api.StartTaskResponseObject, error) {
	err := s.jobs.Start(ctx, request.TaskId)
	if errors.Is(err, jobs.ErrNotFound) {
		return api.StartTask404JSONResponse{}, nil
	}
	if err != nil {
		return nil, err
	}

	return api.StartTask204Response{}, nil
}

func (s *Server) StopTask(ctx context.Context, request api.StopTaskRequestObject) (api.StopTaskResponseObject, error) {
	err := s.jobs.Cancel(ctx, request.TaskId)
	if errors.Is(err, jobs.ErrNotFound) {
		return api.StopTask404JSONResponse{}, nil
	}
	if err != nil {
		return nil, err
	}

	return api.StopTask204Response{}, nil
}

func (s *Server) UpdateTask(ctx context.Context, request api.UpdateTaskRequestObject) (api.UpdateTaskResponseObject, error) {
	infos := apiutil.Body(request.JSONBody, request.ApplicationWildcardPlusJSONBody)
	triggers := make([]jobs.Trigger, 0)
	if infos != nil {
		for _, info := range *infos {
			triggers = append(triggers, trigger(info))
		}
	}

	err := s.jobs.Schedule(ctx, request.TaskId, triggers)
	if errors.Is(err, jobs.ErrNotFound) {
		return api.UpdateTask404JSONResponse{}, nil
	}
	if err != nil {
		return nil, err
	}

	return api.UpdateTask204Response{}, nil
}
