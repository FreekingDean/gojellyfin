package userlibrary

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/dto"
)

// Intros are cinema-mode pre-rolls, which upstream sources from a provider
// plugin. Nothing here supplies them, and a client asking before playback
// wants an empty list rather than an error.
func (s *Server) GetIntros(ctx context.Context, request api.GetIntrosRequestObject) (api.GetIntrosResponseObject, error) {
	return api.GetIntros200JSONResponse(dto.EmptyItems()), nil
}

// The scanner records one item per media file and no extras beside it, so an
// item never has trailers or special features to report.
func (s *Server) GetLocalTrailers(ctx context.Context, request api.GetLocalTrailersRequestObject) (api.GetLocalTrailersResponseObject, error) {
	return api.GetLocalTrailers200JSONResponse{}, nil
}

func (s *Server) GetSpecialFeatures(ctx context.Context, request api.GetSpecialFeaturesRequestObject) (api.GetSpecialFeaturesResponseObject, error) {
	return api.GetSpecialFeatures200JSONResponse{}, nil
}
