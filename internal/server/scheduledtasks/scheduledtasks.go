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
	disabledOnly := !apiutil.Deref(apiutil.OrElse(request.Params.IsEnabled, true))

	infos := make([]api.TaskInfo, 0)
	if hiddenOnly || disabledOnly {
		return api.GetTasks200JSONResponse(infos), nil
	}

	summaries, err := s.jobs.Summaries(ctx)
	if err != nil {
		return nil, err
	}
	for _, summary := range summaries {
		infos = append(infos, taskInfo(summary))
	}

	return api.GetTasks200JSONResponse(infos), nil
}

func (s *Server) GetTask(ctx context.Context, request api.GetTaskRequestObject) (api.GetTaskResponseObject, error) {
	summary, err := s.jobs.Summary(ctx, request.TaskId)
	if errors.Is(err, jobs.ErrNotFound) {
		return api.GetTask404JSONResponse{}, nil
	}
	if err != nil {
		return nil, err
	}

	return api.GetTask200JSONResponse(taskInfo(summary)), nil
}

func (s *Server) StartTask(ctx context.Context, request api.StartTaskRequestObject) (api.StartTaskResponseObject, error) {
	_, err := s.jobs.Start(ctx, request.TaskId)
	if errors.Is(err, jobs.ErrNotFound) {
		return api.StartTask404JSONResponse{}, nil
	}
	if err != nil {
		return nil, err
	}

	return api.StartTask204Response{}, nil
}

func (s *Server) StopTask(ctx context.Context, request api.StopTaskRequestObject) (api.StopTaskResponseObject, error) {
	summary, err := s.jobs.Summary(ctx, request.TaskId)
	if errors.Is(err, jobs.ErrNotFound) {
		return api.StopTask404JSONResponse{}, nil
	}
	if err != nil {
		return nil, err
	}

	if summary.Active != nil {
		if err := s.jobs.Cancel(ctx, summary.Active.ID); err != nil && !errors.Is(err, jobs.ErrLeaseLost) {
			return nil, err
		}
	}

	return api.StopTask204Response{}, nil
}

func (s *Server) UpdateTask(ctx context.Context, request api.UpdateTaskRequestObject) (api.UpdateTaskResponseObject, error) {
	body := apiutil.Body(request.JSONBody, request.ApplicationWildcardPlusJSONBody)

	err := s.jobs.SetTriggers(ctx, request.TaskId, triggers(apiutil.Deref(body)))
	if errors.Is(err, jobs.ErrNotFound) {
		return api.UpdateTask404JSONResponse{}, nil
	}
	if err != nil {
		return nil, err
	}

	return api.UpdateTask204Response{}, nil
}
