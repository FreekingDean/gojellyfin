package studios

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

type Server struct{}

func New() *Server {
	return &Server{}
}

func (s *Server) GetStudios(ctx context.Context, request api.GetStudiosRequestObject) (api.GetStudiosResponseObject, error) {
	return api.GetStudios200JSONResponse{
		Items:            &[]api.BaseItemDto{},
		StartIndex:       apiutil.Ptr(int32(0)),
		TotalRecordCount: apiutil.Ptr(int32(0)),
	}, nil
}
