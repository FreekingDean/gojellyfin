package health

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/health"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
)

type Server struct {
	health *health.Service
}

func New(service *health.Service) *Server {
	return &Server{health: service}
}

func (s *Server) GoJellyfinGetHealth(
	ctx context.Context,
	_ api.GoJellyfinGetHealthRequestObject,
) (api.GoJellyfinGetHealthResponseObject, error) {
	report, err := s.health.Report(ctx)
	if err != nil {
		return nil, err
	}

	return api.GoJellyfinGetHealth200JSONResponse(healthDto(report)), nil
}
