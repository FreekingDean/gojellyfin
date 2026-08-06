package livetv

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/dtos"
)

// Live TV is not implemented. These two exist only because the web home screen
// polls them on a retry loop when they 501; everything else in the tag stays
// unimplemented.
type Server struct{}

func New() *Server {
	return &Server{}
}

func (s *Server) GetLiveTvInfo(ctx context.Context, request api.GetLiveTvInfoRequestObject) (api.GetLiveTvInfoResponseObject, error) {
	return api.GetLiveTvInfo200JSONResponse{
		IsEnabled:    dtos.Ptr(false),
		Services:     &[]api.LiveTvServiceInfo{},
		EnabledUsers: &[]string{},
	}, nil
}

func (s *Server) GetRecommendedPrograms(ctx context.Context, request api.GetRecommendedProgramsRequestObject) (api.GetRecommendedProgramsResponseObject, error) {
	return api.GetRecommendedPrograms200JSONResponse{
		Items:            &[]api.BaseItemDto{},
		StartIndex:       dtos.Ptr(int32(0)),
		TotalRecordCount: dtos.Ptr(int32(0)),
	}, nil
}
