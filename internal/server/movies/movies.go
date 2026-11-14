package movies

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
)

type Server struct{}

func New() *Server {
	return &Server{}
}

func (s *Server) GetMovieRecommendations(ctx context.Context, request api.GetMovieRecommendationsRequestObject) (api.GetMovieRecommendationsResponseObject, error) {
	return api.GetMovieRecommendations200JSONResponse{}, nil
}
