package backup

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
)

type Server struct{}

func New() *Server {
	return &Server{}
}

func (s *Server) ListBackups(ctx context.Context, request api.ListBackupsRequestObject) (api.ListBackupsResponseObject, error) {
	return api.ListBackups200JSONResponse{}, nil
}
