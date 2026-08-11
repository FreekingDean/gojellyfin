package search

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	serveritems "github.com/FreekingDean/gojellyfin/internal/server/items"
)

type Server struct {
	items *items.Service
}

func New(items *items.Service) *Server {
	return &Server{items: items}
}

func (s *Server) GetSearchHints(ctx context.Context, request api.GetSearchHintsRequestObject) (api.GetSearchHintsResponseObject, error) {
	query := items.ItemQuery{
		SearchTerm: request.Params.SearchTerm,
		StartIndex: int(apiutil.Deref(request.Params.StartIndex)),
		Limit:      int(apiutil.Deref(request.Params.Limit)),
		SortBy:     []string{"SortName"},
	}
	query.Kinds = serveritems.Kinds(request.Params.IncludeItemTypes)
	if request.Params.ParentId != nil {
		query.ParentID = request.Params.ParentId
	}

	records, total, err := s.items.QueryItems(ctx, query)
	if err != nil {
		return nil, err
	}

	hints := make([]api.SearchHint, 0, len(records))
	for _, record := range records {
		hints = append(hints, searchHint(record, request.Params.SearchTerm))
	}

	return api.GetSearchHints200JSONResponse{
		SearchHints:      &hints,
		TotalRecordCount: apiutil.Ptr(int32(total)),
	}, nil
}

func searchHint(item *items.Item, term string) api.SearchHint {
	kind := api.BaseItemKind(item.Kind)

	hint := api.SearchHint{
		ItemId:            &item.ID,
		Id:                &item.ID,
		Name:              apiutil.Ptr(item.Name),
		MatchedTerm:       apiutil.Ptr(term),
		Type:              &kind,
		IsFolder:          apiutil.Ptr(item.IsFolder),
		IndexNumber:       item.IndexNumber,
		ParentIndexNumber: item.ParentIndexNumber,
		ProductionYear:    item.ProductionYear,
		RunTimeTicks:      item.RunTimeTicks,
	}

	if !item.IsFolder {
		hint.MediaType = apiutil.Ptr(api.MediaType(item.MediaType))
	}

	return hint
}
