package clientlog

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

const logged = 64 << 10

type Server struct{}

func New() *Server {
	return &Server{}
}

func (s *Server) LogFile(ctx context.Context, request api.LogFileRequestObject) (api.LogFileResponseObject, error) {
	authorization := auth.AuthorizationFrom(ctx)
	name := fmt.Sprintf("upload_%s_%s_%s.log", authorization.Client, authorization.Version, time.Now().UTC().Format("20060102150405"))

	log.Printf("client log %s from %s on %s:\n%s", name, authorization.Client, authorization.Device, document(request.Body))

	return api.LogFile200JSONResponse{FileName: apiutil.Ptr(name)}, nil
}

func document(body *api.LogFileTextRequestBody) string {
	if body == nil {
		return ""
	}
	if len(*body) > logged {
		return string((*body)[:logged])
	}

	return string(*body)
}
