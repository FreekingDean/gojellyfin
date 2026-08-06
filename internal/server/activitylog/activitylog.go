package activitylog

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
)

type Server struct{}

func New() *Server {
	return &Server{}
}

func (s *Server) GetLogEntries(ctx context.Context, request api.GetLogEntriesRequestObject) (api.GetLogEntriesResponseObject, error) {
	return api.GetLogEntries200JSONResponse{}, nil
}
