package items

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/config"
	"github.com/FreekingDean/gojellyfin/internal/http/middleware"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
)

var folderTypes = map[string]bool{
	"Series":           true,
	"Season":           true,
	"Folder":           true,
	"CollectionFolder": true,
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
	item, err := s.store.ItemByID(ctx, request.ItemId)
	if err != nil {
		return api.GetItem403Response{}, nil
	}

	dtos, err := s.itemDtos(ctx, []items.Item{*item})
	if err != nil {
		return nil, err
	}

	return api.GetItem200JSONResponse(dtos[0]), nil
}

func (s *Server) GetRootFolder(ctx context.Context, request api.GetRootFolderRequestObject) (api.GetRootFolderResponseObject, error) {
	return api.GetRootFolder200JSONResponse{
		Id:       uid(config.RootFolderID),
		Name:     ptr("Media Folders"),
		ServerId: ptr(config.ServerID),
		Type:     ptr(api.BaseItemKindFolder),
		IsFolder: ptr(true),
	}, nil
}

func (s *Server) GetUserViews(ctx context.Context, request api.GetUserViewsRequestObject) (api.GetUserViewsResponseObject, error) {
	libraries, err := s.libraries.ListLibraries(ctx)
	if err != nil {
		return nil, err
	}

	views := make([]api.BaseItemDto, 0, len(libraries))
	for _, library := range libraries {
		views = append(views, libraryView(&library))
	}

	return api.GetUserViews200JSONResponse{
		Items:            &views,
		StartIndex:       ptr(int32(0)),
		TotalRecordCount: ptr(int32(len(views))),
	}, nil
}

func (s *Server) GetLatestMedia(ctx context.Context, request api.GetLatestMediaRequestObject) (api.GetLatestMediaResponseObject, error) {
	query := items.ItemQuery{
		Types:      []string{"Movie", "Series"},
		SortBy:     []string{"DateCreated"},
		Descending: true,
		Limit:      int(deref(orElse(request.Params.Limit, int32(20)))),
	}
	if request.Params.ParentId != nil {
		query.LibraryID = request.Params.ParentId
	}

	records, _, err := s.store.QueryItems(ctx, query)
	if err != nil {
		return nil, err
	}

	dtos, err := s.itemDtos(ctx, records)
	if err != nil {
		return nil, err
	}

	return api.GetLatestMedia200JSONResponse(dtos), nil
}

func (s *Server) itemQuery(ctx context.Context, params api.GetItemsParams) (items.ItemQuery, error) {
	query := items.ItemQuery{
		SearchTerm: deref(params.SearchTerm),
		StartIndex: int(deref(params.StartIndex)),
		Limit:      int(deref(params.Limit)),
		Descending: descending(params.SortOrder),
		SortBy:     sortFields(params.SortBy),
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
			query.TopLevel = !deref(params.Recursive)
		default:
			query.ParentID = params.ParentId
		}
	}

	return query, nil
}

func (s *Server) queryResult(ctx context.Context, query items.ItemQuery) (api.BaseItemDtoQueryResult, error) {
	records, total, err := s.store.QueryItems(ctx, query)
	if err != nil {
		return api.BaseItemDtoQueryResult{}, err
	}

	dtos, err := s.itemDtos(ctx, records)
	if err != nil {
		return api.BaseItemDtoQueryResult{}, err
	}

	return api.BaseItemDtoQueryResult{
		Items:            &dtos,
		StartIndex:       ptr(int32(query.StartIndex)),
		TotalRecordCount: ptr(int32(total)),
	}, nil
}

func (s *Server) itemDtos(ctx context.Context, records []items.Item) ([]api.BaseItemDto, error) {
	folderIDs := make([]uuid.UUID, 0, len(records))
	itemIDs := make([]uuid.UUID, 0, len(records))
	for _, item := range records {
		itemIDs = append(itemIDs, item.ID)
		if folderTypes[item.Type] {
			folderIDs = append(folderIDs, item.ID)
		}
	}

	counts, err := s.store.CountChildren(ctx, folderIDs)
	if err != nil {
		return nil, err
	}

	userData := map[uuid.UUID]items.Datum{}
	if userID := middleware.UserID(ctx); userID != uuid.Nil {
		if userData, err = s.store.ListUserItemData(ctx, userID, itemIDs); err != nil {
			return nil, err
		}
	}

	dtos := make([]api.BaseItemDto, 0, len(records))
	for _, item := range records {
		dto := itemDto(&item, counts[item.ID])
		datum, ok := userData[item.ID]
		if !ok {
			datum = items.Datum{ItemID: item.ID}
		}
		dto.UserData = ptr(userItemDataDto(&datum))
		dtos = append(dtos, dto)
	}

	return dtos, nil
}

func itemDto(item *items.Item, childCount int32) api.BaseItemDto {
	kind := api.BaseItemKind(item.Type)
	isFolder := folderTypes[item.Type]

	dto := api.BaseItemDto{
		Id:                &item.ID,
		ServerId:          ptr(config.ServerID),
		Name:              ptr(item.Name),
		SortName:          ptr(item.SortName),
		Type:              &kind,
		Path:              ptr(item.Path),
		IsFolder:          ptr(isFolder),
		ParentId:          item.ParentID,
		IndexNumber:       item.IndexNumber,
		ParentIndexNumber: item.ParentIndexNumber,
		ProductionYear:    item.ProductionYear,
		PremiereDate:      item.PremiereDate,
		RunTimeTicks:      item.RunTimeTicks,
		DateCreated:       ptr(item.CreatedAt),
		LocationType:      ptr(api.FileSystem),
		ImageTags:         &map[string]string{},
		BackdropImageTags: &[]string{},
	}

	if item.Overview != "" {
		dto.Overview = ptr(item.Overview)
	}
	if isFolder {
		dto.ChildCount = ptr(childCount)
	} else {
		dto.MediaType = ptr(api.MediaTypeVideo)
	}

	return dto
}

func libraryView(library *libraries.Library) api.BaseItemDto {
	collectionType := api.CollectionType(library.CollectionType)

	return api.BaseItemDto{
		Id:                &library.ID,
		ServerId:          ptr(config.ServerID),
		Name:              ptr(library.Name),
		SortName:          ptr(strings.ToLower(library.Name)),
		Type:              ptr(api.BaseItemKindCollectionFolder),
		CollectionType:    &collectionType,
		IsFolder:          ptr(true),
		LocationType:      ptr(api.FileSystem),
		ImageTags:         &map[string]string{},
		BackdropImageTags: &[]string{},
	}
}

func descending(order *[]api.SortOrder) bool {
	if order == nil {
		return false
	}

	for _, value := range *order {
		if value == api.Descending {
			return true
		}
	}

	return false
}

func sortFields(sortBy *[]api.ItemSortBy) []string {
	if sortBy == nil {
		return nil
	}

	fields := make([]string, 0, len(*sortBy))
	for _, field := range *sortBy {
		fields = append(fields, string(field))
	}

	return fields
}
