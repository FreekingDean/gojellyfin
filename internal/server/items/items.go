package items

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/discover"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
	"github.com/FreekingDean/gojellyfin/internal/requests"
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
	discover  *discover.Service
	requests  *requests.Service
	collages  dto.Collages
}

func New(items *items.Service, libraries *libraries.Service, shelves *shelves.Service, discover *discover.Service, requests *requests.Service, policies dto.Access, collages dto.Collages) *Server {
	return &Server{items: items, libraries: libraries, shelves: shelves, discover: discover, requests: requests, policies: policies, collages: collages}
}

func (s *Server) shelved(ctx context.Context, shelf shelves.Shelf, limit int, ungrouped bool) ([]api.BaseItemDto, error) {
	latest, err := s.shelves.Latest(ctx, shelf, limit, ungrouped)
	if err != nil {
		return nil, err
	}

	return dto.LatestDtos(ctx, s.items, latest)
}

func (s *Server) unowned(ctx context.Context, id uuid.UUID) (api.BaseItemDto, bool, error) {
	title, found, err := s.discover.Title(ctx, id)
	if err != nil || !found {
		return api.BaseItemDto{}, false, err
	}

	held, err := s.items.ItemByKey(ctx, title.Key)
	if err != nil || held.DeletedAt != nil {
		converted, err := dto.UnownedDtos(ctx, s.requests, []discover.Title{title})
		if err != nil {
			return api.BaseItemDto{}, false, err
		}

		return converted[0], true, nil
	}

	converted, err := dto.ItemDtos(ctx, s.items, []*items.Item{held})
	if err != nil {
		return api.BaseItemDto{}, false, err
	}

	return converted[0], true, nil
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
	if err != nil || !found {
		return api.BaseItemDto{}, found, err
	}

	view, err := dto.ShelfView(ctx, s.collages, shelf)

	return view, true, err
}

func (s *Server) GetItems(ctx context.Context, request api.GetItemsRequestObject) (api.GetItemsResponseObject, error) {
	shelf, found, err := s.shelf(ctx, request.Params.ParentId)
	if err != nil {
		return nil, err
	}
	if found {
		converted, err := s.shelved(ctx, shelf, int(apiutil.OrElse(request.Params.Limit, int32(shelfPage))), true)
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

	wanted, err := s.searched(ctx, request.Params)
	if err != nil {
		return nil, err
	}
	if len(wanted) > 0 {
		held := append(apiutil.Deref(result.Items), wanted...)
		result.Items = &held
		result.TotalRecordCount = apiutil.Ptr(apiutil.Deref(result.TotalRecordCount) + int32(len(wanted)))
	}

	return api.GetItems200JSONResponse(result), nil
}

func (s *Server) GetItem(ctx context.Context, request api.GetItemRequestObject) (api.GetItemResponseObject, error) {
	item, err := s.items.ItemByID(ctx, items.Everyone, request.ItemId)
	if err != nil {
		if title, found, err := s.unowned(ctx, request.ItemId); err != nil || found {
			return api.GetItem200JSONResponse(title), err
		}

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

	if found {
		converted, err := s.shelved(ctx, shelf, limit, ungrouped)
		if err != nil {
			return nil, err
		}

		return api.GetLatestMedia200JSONResponse(converted), nil
	}

	latest, err := s.items.LatestItems(ctx, items.LatestQuery{
		Viewer:    viewer,
		LibraryID: request.Params.ParentId,
		Order:     items.LatestAdded,
		Now:       time.Now(),
		Limit:     limit,
		Ungrouped: ungrouped,
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
