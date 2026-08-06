package syncplay

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
)

type Server struct{}

func New() *Server {
	return &Server{}
}

func (s *Server) SyncPlayGetGroups(ctx context.Context, request api.SyncPlayGetGroupsRequestObject) (api.SyncPlayGetGroupsResponseObject, error) {
	return api.SyncPlayGetGroups200JSONResponse{}, nil
}
