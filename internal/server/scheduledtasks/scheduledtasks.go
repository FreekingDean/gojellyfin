package scheduledtasks

import (
	"context"
	"errors"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/tasks"
)

type Server struct {
	registry *tasks.Registry
}

func New(registry *tasks.Registry) *Server {
	return &Server{registry: registry}
}

func (s *Server) GetTasks(ctx context.Context, request api.GetTasksRequestObject) (api.GetTasksResponseObject, error) {
	hiddenOnly := apiutil.Deref(request.Params.IsHidden)
	disabledOnly := !apiutil.Deref(apiutil.OrElse(request.Params.IsEnabled, true))

	infos := make([]api.TaskInfo, 0)
	if hiddenOnly || disabledOnly {
		return api.GetTasks200JSONResponse(infos), nil
	}

	for _, info := range s.registry.Tasks() {
		infos = append(infos, taskInfo(info))
	}

	return api.GetTasks200JSONResponse(infos), nil
}

func (s *Server) GetTask(ctx context.Context, request api.GetTaskRequestObject) (api.GetTaskResponseObject, error) {
	info, err := s.registry.Task(request.TaskId)
	if errors.Is(err, tasks.ErrNotFound) {
		return api.GetTask404JSONResponse{}, nil
	}
	if err != nil {
		return nil, err
	}

	return api.GetTask200JSONResponse(taskInfo(info)), nil
}

func (s *Server) StartTask(ctx context.Context, request api.StartTaskRequestObject) (api.StartTaskResponseObject, error) {
	err := s.registry.Start(request.TaskId)
	if errors.Is(err, tasks.ErrNotFound) {
		return api.StartTask404JSONResponse{}, nil
	}
	if err != nil {
		return nil, err
	}

	return api.StartTask204Response{}, nil
}

func (s *Server) StopTask(ctx context.Context, request api.StopTaskRequestObject) (api.StopTaskResponseObject, error) {
	err := s.registry.Stop(request.TaskId)
	if errors.Is(err, tasks.ErrNotFound) {
		return api.StopTask404JSONResponse{}, nil
	}
	if err != nil {
		return nil, err
	}

	return api.StopTask204Response{}, nil
}

func (s *Server) UpdateTask(ctx context.Context, request api.UpdateTaskRequestObject) (api.UpdateTaskResponseObject, error) {
	body := apiutil.Body(request.JSONBody, request.ApplicationWildcardPlusJSONBody)

	err := s.registry.SetTriggers(request.TaskId, triggers(apiutil.Deref(body)))
	if errors.Is(err, tasks.ErrNotFound) {
		return api.UpdateTask404JSONResponse{}, nil
	}
	if err != nil {
		return nil, err
	}

	return api.UpdateTask204Response{}, nil
}
