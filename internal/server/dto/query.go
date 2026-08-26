package dto

import (
	"context"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

func QueryResult(ctx context.Context, store *items.Service, collections *libraries.Service, params api.GetItemsParams) (api.BaseItemDtoQueryResult, error) {
	query := ItemQuery(ctx, collections, params)

	records, total, err := store.QueryItems(ctx, query)
	if err != nil {
		return api.BaseItemDtoQueryResult{}, err
	}

	converted, err := ItemDtos(ctx, store, records)
	if err != nil {
		return api.BaseItemDtoQueryResult{}, err
	}

	return api.BaseItemDtoQueryResult{
		Items:            &converted,
		StartIndex:       apiutil.Ptr(int32(query.StartIndex)),
		TotalRecordCount: apiutil.Ptr(int32(total)),
	}, nil
}

func ItemQuery(ctx context.Context, collections *libraries.Service, params api.GetItemsParams) items.ItemQuery {
	query := items.ItemQuery{
		SearchTerm: apiutil.Deref(params.SearchTerm),
		StartIndex: int(apiutil.Deref(params.StartIndex)),
		Limit:      int(apiutil.Deref(params.Limit)),
		Descending: Descending(params.SortOrder),
		SortBy:     SortFields(params.SortBy),
		Kinds:      Kinds(params.IncludeItemTypes),
		MediaTypes: MediaTypes(params.MediaTypes),
	}

	if params.Ids != nil {
		query.IDs = *params.Ids
	}
	if params.ArtistIds != nil {
		query.ArtistIDs = *params.ArtistIds
	}
	if params.AlbumArtistIds != nil {
		query.ArtistIDs = append(query.ArtistIDs, *params.AlbumArtistIds...)
	}
	if params.AlbumIds != nil {
		query.AlbumIDs = *params.AlbumIds
	}
	if params.GenreIds != nil {
		query.GenreIDs = *params.GenreIds
	}

	ScopeToParent(ctx, collections, &query, params.ParentId, apiutil.Deref(params.Recursive))

	return query
}

// A client navigates into a library by its id the same way it navigates into a
// folder, and only one of the two is an item row.
func ScopeToParent(ctx context.Context, collections *libraries.Service, query *items.ItemQuery, parentID *uuid.UUID, recursive bool) {
	if parentID == nil {
		return
	}

	library, err := collections.Library(ctx, *parentID)
	if err != nil {
		query.ParentID = parentID

		return
	}

	query.LibraryID = &library.ID
	query.TopLevel = !recursive
}

func Descending(order *[]api.SortOrder) bool {
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

func SortFields(sortBy *[]api.ItemSortBy) []string {
	if sortBy == nil {
		return nil
	}

	fields := make([]string, 0, len(*sortBy))
	for _, field := range *sortBy {
		fields = append(fields, string(field))
	}

	return fields
}

func EmptyItems() api.BaseItemDtoQueryResult {
	return api.BaseItemDtoQueryResult{
		Items:            &[]api.BaseItemDto{},
		StartIndex:       apiutil.Ptr(int32(0)),
		TotalRecordCount: apiutil.Ptr(int32(0)),
	}
}
