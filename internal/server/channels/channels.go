package channels

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/dto"
)

// Channels come from plugins, which are not supported.
type Server struct{}

func New() *Server {
	return &Server{}
}

func (s *Server) GetChannels(ctx context.Context, request api.GetChannelsRequestObject) (api.GetChannelsResponseObject, error) {
	return api.GetChannels200JSONResponse(dto.EmptyItems()), nil
}
