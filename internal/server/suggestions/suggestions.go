package suggestions

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/dto"
)

type Server struct {
	policies  dto.Access
	items     *items.Service
	libraries *libraries.Service
}

func New(items *items.Service, libraries *libraries.Service, policies dto.Access) *Server {
	return &Server{items: items, libraries: libraries, policies: policies}
}

func (s *Server) GetSuggestions(ctx context.Context, request api.GetSuggestionsRequestObject) (api.GetSuggestionsResponseObject, error) {
	result, err := dto.QueryResult(ctx, s.items, s.libraries, s.policies, api.GetItemsParams{
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
