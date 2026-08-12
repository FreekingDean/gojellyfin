package timesync

import (
	"context"
	"time"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

type Server struct{}

func New() *Server {
	return &Server{}
}

func (s *Server) GetUtcTime(ctx context.Context, request api.GetUtcTimeRequestObject) (api.GetUtcTimeResponseObject, error) {
	received := time.Now().UTC()

	return api.GetUtcTime200JSONResponse{
		RequestReceptionTime:     apiutil.Ptr(received),
		ResponseTransmissionTime: apiutil.Ptr(time.Now().UTC()),
	}, nil
}
