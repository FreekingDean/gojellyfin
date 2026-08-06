package dashboard

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
)

// Configuration pages are contributed by plugins, and plugins are not supported.
type Server struct{}

func New() *Server {
	return &Server{}
}

func (s *Server) GetConfigurationPages(ctx context.Context, request api.GetConfigurationPagesRequestObject) (api.GetConfigurationPagesResponseObject, error) {
	return api.GetConfigurationPages200JSONResponse([]api.ConfigurationPageInfo{}), nil
}
