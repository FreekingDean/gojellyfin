package suggestions

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
	serveritems "github.com/FreekingDean/gojellyfin/internal/server/items"
)

type Server struct {
	items *serveritems.Server
}

func New(items *serveritems.Server) *Server {
	return &Server{items: items}
}

func (s *Server) GetSuggestions(ctx context.Context, request api.GetSuggestionsRequestObject) (api.GetSuggestionsResponseObject, error) {
	result, err := s.items.QueryResult(ctx, api.GetItemsParams{
		IncludeItemTypes: request.Params.Type,
		MediaTypes:       request.Params.MediaType,
		SortBy:           &[]api.ItemSortBy{api.ItemSortByRandom},
		StartIndex:       request.Params.StartIndex,
		Limit:            request.Params.Limit,
	})
	if err != nil {
		return nil, err
	}

	return api.GetSuggestions200JSONResponse(result), nil
}
