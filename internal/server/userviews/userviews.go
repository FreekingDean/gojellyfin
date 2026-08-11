package userviews

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/libraries"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/server/dto"
	librarymodal "github.com/FreekingDean/gojellyfin/internal/store/library"
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
		views = append(views, dto.LibraryView(library))
	}

	return api.GetUserViews200JSONResponse{
		Items:            &views,
		StartIndex:       apiutil.Ptr(int32(0)),
		TotalRecordCount: apiutil.Ptr(int32(len(views))),
	}, nil
}

var groupable = map[libraries.CollectionType]bool{
	librarymodal.CollectionTypeMovies:  true,
	librarymodal.CollectionTypeTvshows: true,
	librarymodal.CollectionTypeMixed:   true,
}

func (s *Server) GetGroupingOptions(ctx context.Context, request api.GetGroupingOptionsRequestObject) (api.GetGroupingOptionsResponseObject, error) {
	records, err := s.libraries.ListLibraries(ctx)
	if err != nil {
		return nil, err
	}

	options := make([]api.SpecialViewOptionDto, 0, len(records))
	for _, library := range records {
		if !groupable[library.CollectionType] {
			continue
		}
		options = append(options, api.SpecialViewOptionDto{
			Id:   apiutil.Ptr(library.ID.String()),
			Name: apiutil.Ptr(library.Name),
		})
	}

	return api.GetGroupingOptions200JSONResponse(options), nil
}
