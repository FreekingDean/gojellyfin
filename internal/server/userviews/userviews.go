package userviews

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/libraries"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	serveritems "github.com/FreekingDean/gojellyfin/internal/server/items"
)

type Server struct {
	libraries *libraries.Service
}

func New(libraries *libraries.Service) *Server {
	return &Server{libraries: libraries}
}

func (s *Server) GetUserViews(ctx context.Context, request api.GetUserViewsRequestObject) (api.GetUserViewsResponseObject, error) {
	libraries, err := s.libraries.ListLibraries(ctx)
	if err != nil {
		return nil, err
	}

	views := make([]api.BaseItemDto, 0, len(libraries))
	for _, library := range libraries {
		views = append(views, serveritems.LibraryView(&library))
	}

	return api.GetUserViews200JSONResponse{
		Items:            &views,
		StartIndex:       apiutil.Ptr(int32(0)),
		TotalRecordCount: apiutil.Ptr(int32(len(views))),
	}, nil
}
