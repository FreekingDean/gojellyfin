package dto

import (
	"context"

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
		MediaTypes: Accepted(params.MediaTypes, items.ValidMediaType),
	}

	if params.Ids != nil {
		query.IDs = *params.Ids
	}

	if params.ParentId != nil {
		if library, err := collections.Library(ctx, *params.ParentId); err == nil {
			query.LibraryID = &library.ID
			query.TopLevel = !apiutil.Deref(params.Recursive)
		} else {
			query.ParentID = params.ParentId
		}
	}

	return query
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
