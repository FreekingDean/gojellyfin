package items

import (
	"context"
	"time"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/server/dto"
)

type Server struct {
	policies  dto.Access
	items     *items.Service
	libraries *libraries.Service
	collages  dto.Collages
}

func New(items *items.Service, libraries *libraries.Service, policies dto.Access, collages dto.Collages) *Server {
	return &Server{items: items, libraries: libraries, policies: policies, collages: collages}
}

func (s *Server) GetItems(ctx context.Context, request api.GetItemsRequestObject) (api.GetItemsResponseObject, error) {
	result, err := dto.QueryResult(ctx, s.items, s.libraries, s.policies, request.Params)
	if err != nil {
		return nil, err
	}

	return api.GetItems200JSONResponse(result), nil
}

func (s *Server) GetItem(ctx context.Context, request api.GetItemRequestObject) (api.GetItemResponseObject, error) {
	item, err := s.items.ItemByID(ctx, items.Everyone, request.ItemId)
	if err != nil {
		library, libraryErr := s.libraries.Library(ctx, request.ItemId)
		if libraryErr != nil {
			return api.GetItem403Response{}, nil
		}

		view, err := dto.LibraryView(ctx, s.collages, library)
		if err != nil {
			return nil, err
		}

		return api.GetItem200JSONResponse(view), nil
	}

	converted, err := dto.ItemDtos(ctx, s.items, []*items.Item{item})
	if err != nil {
		return nil, err
	}

	return api.GetItem200JSONResponse(converted[0]), nil
}

func (s *Server) GetRootFolder(ctx context.Context, request api.GetRootFolderRequestObject) (api.GetRootFolderResponseObject, error) {
	return api.GetRootFolder200JSONResponse(dto.RootView()), nil
}

func (s *Server) GetLatestMedia(ctx context.Context, request api.GetLatestMediaRequestObject) (api.GetLatestMediaResponseObject, error) {
	viewer, err := dto.ViewerFor(ctx, s.policies)
	if err != nil {
		return nil, err
	}

	latest, err := s.items.LatestItems(ctx, items.LatestQuery{
		Viewer:    viewer,
		LibraryID: request.Params.ParentId,
		Order:     items.LatestAdded,
		Now:       time.Now(),
		Limit:     int(apiutil.OrElse(request.Params.Limit, int32(20))),
		Ungrouped: !apiutil.OrElse(request.Params.GroupItems, true),
	})
	if err != nil {
		return nil, err
	}

	converted, err := dto.LatestDtos(ctx, s.items, latest)
	if err != nil {
		return nil, err
	}

	return api.GetLatestMedia200JSONResponse(converted), nil
}
