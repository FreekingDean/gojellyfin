package userviews

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/server/dto"
	"github.com/FreekingDean/gojellyfin/internal/shelves"
)

type Server struct {
	libraries *libraries.Service
	shelves   *shelves.Service
	policies  dto.Access
	collages  dto.Collages
}

func New(libraries *libraries.Service, shelves *shelves.Service, policies dto.Access, collages dto.Collages) *Server {
	return &Server{libraries: libraries, shelves: shelves, policies: policies, collages: collages}
}

func (s *Server) GetUserViews(ctx context.Context, request api.GetUserViewsRequestObject) (api.GetUserViewsResponseObject, error) {
	records, err := s.libraries.ListLibraries(ctx)
	if err != nil {
		return nil, err
	}

	views, err := dto.LibraryViews(ctx, s.collages, records)
	if err != nil {
		return nil, err
	}

	viewer, err := dto.ViewerFor(ctx, s.policies)
	if err != nil {
		return nil, err
	}

	shown, err := s.shelves.Shelves(ctx, viewer, auth.UserID(ctx))
	if err != nil {
		return nil, err
	}
	for _, shelf := range shown {
		views = append(views, dto.ShelfView(shelf))
	}

	return api.GetUserViews200JSONResponse{
		Items:            &views,
		StartIndex:       apiutil.Ptr(int32(0)),
		TotalRecordCount: apiutil.Ptr(int32(len(views))),
	}, nil
}

func (s *Server) GetGroupingOptions(ctx context.Context, request api.GetGroupingOptionsRequestObject) (api.GetGroupingOptionsResponseObject, error) {
	records, err := s.libraries.GroupableLibraries(ctx)
	if err != nil {
		return nil, err
	}

	options := make([]api.SpecialViewOptionDto, 0, len(records))
	for _, library := range records {
		options = append(options, groupingOption(library))
	}

	return api.GetGroupingOptions200JSONResponse(options), nil
}
