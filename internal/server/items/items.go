package items

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/config"
	"github.com/FreekingDean/gojellyfin/internal/http/middleware"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/dtos"
	"github.com/google/uuid"
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
		return api.GetItem403Response{}, nil
	}

	converted, err := s.itemDtos(ctx, []items.Item{*item})
	if err != nil {
		return nil, err
	}

	return api.GetItem200JSONResponse(converted[0]), nil
}

func (s *Server) GetRootFolder(ctx context.Context, request api.GetRootFolderRequestObject) (api.GetRootFolderResponseObject, error) {
	return api.GetRootFolder200JSONResponse{
		Id:       dtos.UID(config.RootFolderID),
		Name:     dtos.Ptr("Media Folders"),
		ServerId: dtos.Ptr(config.ServerID),
		Type:     dtos.Ptr(api.BaseItemKindFolder),
		IsFolder: dtos.Ptr(true),
	}, nil
}

func (s *Server) GetLatestMedia(ctx context.Context, request api.GetLatestMediaRequestObject) (api.GetLatestMediaResponseObject, error) {
	query := items.ItemQuery{
		Types:      []string{"Movie", "Series"},
		SortBy:     []string{"DateCreated"},
		Descending: true,
		Limit:      int(dtos.Deref(dtos.OrElse(request.Params.Limit, int32(20)))),
	}
	if request.Params.ParentId != nil {
		query.LibraryID = request.Params.ParentId
	}

	records, _, err := s.items.QueryItems(ctx, query)
	if err != nil {
		return nil, err
	}

	converted, err := s.itemDtos(ctx, records)
	if err != nil {
		return nil, err
	}

	return api.GetLatestMedia200JSONResponse(converted), nil
}

func (s *Server) itemQuery(ctx context.Context, params api.GetItemsParams) (items.ItemQuery, error) {
	query := items.ItemQuery{
		SearchTerm: dtos.Deref(params.SearchTerm),
		StartIndex: int(dtos.Deref(params.StartIndex)),
		Limit:      int(dtos.Deref(params.Limit)),
		Descending: dtos.Descending(params.SortOrder),
		SortBy:     dtos.SortFields(params.SortBy),
	}

	if params.IncludeItemTypes != nil {
		for _, kind := range *params.IncludeItemTypes {
			query.Types = append(query.Types, string(kind))
		}
	}
	if params.Ids != nil {
		query.IDs = *params.Ids
	}

	if params.ParentId != nil {
		library, err := s.libraries.GetLibrary(ctx, *params.ParentId)
		switch {
		case err == nil:
			query.LibraryID = &library.ID
			query.TopLevel = !dtos.Deref(params.Recursive)
		default:
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

	converted, err := s.itemDtos(ctx, records)
	if err != nil {
		return api.BaseItemDtoQueryResult{}, err
	}

	return api.BaseItemDtoQueryResult{
		Items:            &converted,
		StartIndex:       dtos.Ptr(int32(query.StartIndex)),
		TotalRecordCount: dtos.Ptr(int32(total)),
	}, nil
}

func (s *Server) itemDtos(ctx context.Context, records []items.Item) ([]api.BaseItemDto, error) {
	folderIDs := make([]uuid.UUID, 0, len(records))
	itemIDs := make([]uuid.UUID, 0, len(records))
	for _, item := range records {
		itemIDs = append(itemIDs, item.ID)
		if dtos.FolderTypes[item.Type] {
			folderIDs = append(folderIDs, item.ID)
		}
	}

	counts, err := s.items.CountChildren(ctx, folderIDs)
	if err != nil {
		return nil, err
	}

	userData := map[uuid.UUID]items.Datum{}
	if userID := middleware.UserID(ctx); userID != uuid.Nil {
		if userData, err = s.items.ListUserItemData(ctx, userID, itemIDs); err != nil {
			return nil, err
		}
	}

	converted := make([]api.BaseItemDto, 0, len(records))
	for _, item := range records {
		dto := dtos.ItemDto(&item, counts[item.ID])
		datum, ok := userData[item.ID]
		if !ok {
			datum = items.Datum{ItemID: item.ID}
		}
		dto.UserData = dtos.Ptr(dtos.UserItemDataDto(&datum))
		converted = append(converted, dto)
	}

	return converted, nil
}
