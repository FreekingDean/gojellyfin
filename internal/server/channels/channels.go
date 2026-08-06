package channels

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

// Channels come from plugins, which are not supported.
type Server struct{}

func New() *Server {
	return &Server{}
}

func (s *Server) GetChannels(ctx context.Context, request api.GetChannelsRequestObject) (api.GetChannelsResponseObject, error) {
	return api.GetChannels200JSONResponse{
		Items:            &[]api.BaseItemDto{},
		StartIndex:       apiutil.Ptr(int32(0)),
		TotalRecordCount: apiutil.Ptr(int32(0)),
	}, nil
}
