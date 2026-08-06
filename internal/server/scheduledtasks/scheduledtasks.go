package scheduledtasks

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
)

// There is no task scheduler: the library scan runs at startup and on
// /Library/Refresh, so nothing is schedulable yet.
type Server struct{}

func New() *Server {
	return &Server{}
}

func (s *Server) GetTasks(ctx context.Context, request api.GetTasksRequestObject) (api.GetTasksResponseObject, error) {
	return api.GetTasks200JSONResponse([]api.TaskInfo{}), nil
}
