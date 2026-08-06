package environment

import (
	"context"
	"os"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

type Server struct{}

func New() *Server {
	return &Server{}
}

// Used by the dashboard before it will accept a media path.
func (s *Server) ValidatePath(ctx context.Context, request api.ValidatePathRequestObject) (api.ValidatePathResponseObject, error) {
	req := apiutil.Body(request.JSONBody, request.ApplicationWildcardPlusJSONBody)
	if req == nil || req.Path == nil || *req.Path == "" {
		return api.ValidatePath404JSONResponse{}, nil
	}

	info, err := os.Stat(*req.Path)
	if err != nil {
		return api.ValidatePath404JSONResponse{}, nil
	}
	if apiutil.Deref(req.IsFile) && info.IsDir() {
		return api.ValidatePath404JSONResponse{}, nil
	}

	return api.ValidatePath204Response{}, nil
}
