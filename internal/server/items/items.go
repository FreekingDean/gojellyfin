package items

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/server/dto"
	"github.com/FreekingDean/gojellyfin/internal/shelves"
)

const (
	latestLimit = 20
	shelfPage   = 100
)

type Server struct {
	policies  dto.Access
	items     *items.Service
	libraries *libraries.Service
	shelves   *shelves.Service
	collages  dto.Collages
}

func New(items *items.Service, libraries *libraries.Service, shelves *shelves.Service, policies dto.Access, collages dto.Collages) *Server {
	return &Server{items: items, libraries: libraries, shelves: shelves, policies: policies, collages: collages}
}

func (s *Server) shelf(ctx context.Context, id *uuid.UUID) (shelves.Shelf, bool, error) {
	if id == nil {
		return shelves.Shelf{}, false, nil
	}

	viewer, err := dto.ViewerFor(ctx, s.policies)
	if err != nil {
		return shelves.Shelf{}, false, err
	}

	return s.shelves.Shelf(ctx, *id, viewer, auth.UserID(ctx))
}

func (s *Server) view(ctx context.Context, id uuid.UUID) (api.BaseItemDto, bool, error) {
	if library, err := s.libraries.Library(ctx, id); err == nil {
		view, err := dto.LibraryView(ctx, s.collages, library)

		return view, true, err
	}

	shelf, found, err := s.shelf(ctx, &id)

	return dto.ShelfView(shelf), found, err
}

func (s *Server) GetItems(ctx context.Context, request api.GetItemsRequestObject) (api.GetItemsResponseObject, error) {
	shelf, found, err := s.shelf(ctx, request.Params.ParentId)
	if err != nil {
		return nil, err
	}
	if found {
		latest, err := s.shelves.Latest(ctx, shelf, int(apiutil.OrElse(request.Params.Limit, int32(shelfPage))), true)
		if err != nil {
			return nil, err
		}

		converted, err := dto.LatestDtos(ctx, s.items, latest)
		if err != nil {
			return nil, err
		}

		return api.GetItems200JSONResponse(api.BaseItemDtoQueryResult{
			Items:            &converted,
			StartIndex:       apiutil.Ptr(int32(0)),
			TotalRecordCount: apiutil.Ptr(int32(len(converted))),
		}), nil
	}

	result, err := dto.QueryResult(ctx, s.items, s.libraries, s.policies, request.Params)
	if err != nil {
		return nil, err
	}

	return api.GetItems200JSONResponse(result), nil
}

func (s *Server) GetItem(ctx context.Context, request api.GetItemRequestObject) (api.GetItemResponseObject, error) {
	item, err := s.items.ItemByID(ctx, items.Everyone, request.ItemId)
	if err != nil {
		view, found, err := s.view(ctx, request.ItemId)
		if err != nil {
			return nil, err
		}
		if !found {
			return api.GetItem403Response{}, nil
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

	limit := int(apiutil.OrElse(request.Params.Limit, int32(latestLimit)))
	ungrouped := !apiutil.OrElse(request.Params.GroupItems, true)

	shelf, found, err := s.shelf(ctx, request.Params.ParentId)
	if err != nil {
		return nil, err
	}

	var latest []items.Latest
	if found {
		latest, err = s.shelves.Latest(ctx, shelf, limit, ungrouped)
	} else {
		latest, err = s.items.LatestItems(ctx, items.LatestQuery{
			Viewer:    viewer,
			LibraryID: request.Params.ParentId,
			Order:     items.LatestAdded,
			Now:       time.Now(),
			Limit:     limit,
			Ungrouped: ungrouped,
		})
	}
	if err != nil {
		return nil, err
	}

	converted, err := dto.LatestDtos(ctx, s.items, latest)
	if err != nil {
		return nil, err
	}

	return api.GetLatestMedia200JSONResponse(converted), nil
}
