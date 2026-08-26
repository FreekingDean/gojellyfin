package userlibrary

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/dto"
)

func (s *Server) GetIntros(ctx context.Context, request api.GetIntrosRequestObject) (api.GetIntrosResponseObject, error) {
	return api.GetIntros200JSONResponse(dto.EmptyItems()), nil
}

func (s *Server) GetLocalTrailers(ctx context.Context, request api.GetLocalTrailersRequestObject) (api.GetLocalTrailersResponseObject, error) {
	return api.GetLocalTrailers200JSONResponse{}, nil
}

func (s *Server) GetSpecialFeatures(ctx context.Context, request api.GetSpecialFeaturesRequestObject) (api.GetSpecialFeaturesResponseObject, error) {
	return api.GetSpecialFeatures200JSONResponse{}, nil
}
