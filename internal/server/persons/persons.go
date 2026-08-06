package persons

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/dtos"
)

type Server struct{}

func New() *Server {
	return &Server{}
}

func (s *Server) GetPersons(ctx context.Context, request api.GetPersonsRequestObject) (api.GetPersonsResponseObject, error) {
	return api.GetPersons200JSONResponse{
		Items:            &[]api.BaseItemDto{},
		StartIndex:       dtos.Ptr(int32(0)),
		TotalRecordCount: dtos.Ptr(int32(0)),
	}, nil
}
