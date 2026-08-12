package library

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
)

func (s *Server) GetPhysicalPaths(ctx context.Context, request api.GetPhysicalPathsRequestObject) (api.GetPhysicalPathsResponseObject, error) {
	paths, err := s.libraries.PhysicalPaths(ctx)
	if err != nil {
		return nil, err
	}

	return api.GetPhysicalPaths200JSONResponse(paths), nil
}
