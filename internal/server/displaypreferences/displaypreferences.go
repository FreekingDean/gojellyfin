package displaypreferences

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
)

type Server struct{}

func New() *Server {
	return &Server{}
}

func (s *Server) GetDisplayPreferences(ctx context.Context, request api.GetDisplayPreferencesRequestObject) (api.GetDisplayPreferencesResponseObject, error) {
	return api.GetDisplayPreferences200JSONResponse{}, nil
}
