package items

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/config"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/server/dto"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
)

type Server struct {
	items     *items.Service
	libraries *libraries.Service
}

func New(items *items.Service, libraries *libraries.Service) *Server {
	return &Server{items: items, libraries: libraries}
}

func (s *Server) GetItems(ctx context.Context, request api.GetItemsRequestObject) (api.GetItemsResponseObject, error) {
	query, err := s.itemQuery(ctx, request.Params)
	if err != nil {
		return nil, err
	}

	result, err := s.queryResult(ctx, query)
	if err != nil {
		return nil, err
	}

	return api.GetItems200JSONResponse(result), nil
}

func (s *Server) GetItem(ctx context.Context, request api.GetItemRequestObject) (api.GetItemResponseObject, error) {
	item, err := s.items.ItemByID(ctx, request.ItemId)
	if err != nil {
		// Clients navigate into a library by asking for it as an item, but
		// libraries are rows in their own table rather than items.
		library, libraryErr := s.libraries.Library(ctx, request.ItemId)
		if libraryErr != nil {
			return api.GetItem403Response{}, nil
		}

		return api.GetItem200JSONResponse(dto.LibraryView(library)), nil
	}

	converted, err := dto.ItemDtos(ctx, s.items, []*items.Item{item})
	if err != nil {
		return nil, err
	}

	return api.GetItem200JSONResponse(converted[0]), nil
}

func (s *Server) GetRootFolder(ctx context.Context, request api.GetRootFolderRequestObject) (api.GetRootFolderResponseObject, error) {
	return api.GetRootFolder200JSONResponse{
		Id:       apiutil.UID(config.RootFolderID),
		Name:     apiutil.Ptr("Media Folders"),
		ServerId: apiutil.Ptr(config.ServerID),
		Type:     apiutil.Ptr(api.BaseItemKindFolder),
		IsFolder: apiutil.Ptr(true),
	}, nil
}

func (s *Server) GetLatestMedia(ctx context.Context, request api.GetLatestMediaRequestObject) (api.GetLatestMediaResponseObject, error) {
	query := items.ItemQuery{
		Kinds:      []items.Kind{itemmodal.KindMovie, itemmodal.KindSeries},
		SortBy:     []string{"DateCreated"},
		Descending: true,
		Limit:      int(apiutil.Deref(apiutil.OrElse(request.Params.Limit, int32(20)))),
	}
	if request.Params.ParentId != nil {
		query.LibraryID = request.Params.ParentId
	}

	records, _, err := s.items.QueryItems(ctx, query)
	if err != nil {
		return nil, err
	}

	converted, err := dto.ItemDtos(ctx, s.items, records)
	if err != nil {
		return nil, err
	}

	return api.GetLatestMedia200JSONResponse(converted), nil
}

func (s *Server) itemQuery(ctx context.Context, params api.GetItemsParams) (items.ItemQuery, error) {
	query := items.ItemQuery{
		SearchTerm: apiutil.Deref(params.SearchTerm),
		StartIndex: int(apiutil.Deref(params.StartIndex)),
		Limit:      int(apiutil.Deref(params.Limit)),
		Descending: descending(params.SortOrder),
		SortBy:     sortFields(params.SortBy),
	}

	query.Kinds = dto.Kinds(params.IncludeItemTypes)
	if params.Ids != nil {
		query.IDs = *params.Ids
	}

	if params.ParentId != nil {
		if library, err := s.libraries.Library(ctx, *params.ParentId); err == nil {
			query.LibraryID = &library.ID
			query.TopLevel = !apiutil.Deref(params.Recursive)
		} else {
			query.ParentID = params.ParentId
		}
	}

	return query, nil
}

func (s *Server) queryResult(ctx context.Context, query items.ItemQuery) (api.BaseItemDtoQueryResult, error) {
	records, total, err := s.items.QueryItems(ctx, query)
	if err != nil {
		return api.BaseItemDtoQueryResult{}, err
	}

	converted, err := dto.ItemDtos(ctx, s.items, records)
	if err != nil {
		return api.BaseItemDtoQueryResult{}, err
	}

	return api.BaseItemDtoQueryResult{
		Items:            &converted,
		StartIndex:       apiutil.Ptr(int32(query.StartIndex)),
		TotalRecordCount: apiutil.Ptr(int32(total)),
	}, nil
}
