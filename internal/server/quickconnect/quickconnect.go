package quickconnect

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
)

type Server struct{}

func New() *Server {
	return &Server{}
}

func (s *Server) GetQuickConnectEnabled(ctx context.Context, request api.GetQuickConnectEnabledRequestObject) (api.GetQuickConnectEnabledResponseObject, error) {
	return api.GetQuickConnectEnabled200JSONResponse(true), nil
}
