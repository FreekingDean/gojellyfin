package videos

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
)

type Server struct{}

func New() *Server {
	return &Server{}
}

func (s *Server) GetAttachment(context.Context, api.GetAttachmentRequestObject) (api.GetAttachmentResponseObject, error) {
	return api.GetAttachment404JSONResponse{}, nil
}
