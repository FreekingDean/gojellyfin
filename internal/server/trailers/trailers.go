package trailers

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/dto"
)

type Server struct {
	items     *items.Service
	libraries *libraries.Service
}

func New(items *items.Service, libraries *libraries.Service) *Server {
	return &Server{items: items, libraries: libraries}
}

func (s *Server) GetTrailers(ctx context.Context, request api.GetTrailersRequestObject) (api.GetTrailersResponseObject, error) {
	result, err := dto.QueryResult(ctx, s.items, s.libraries, api.GetItemsParams{
		IncludeItemTypes: &[]api.BaseItemKind{api.BaseItemKindTrailer},
		Ids:              request.Params.Ids,
		ParentId:         request.Params.ParentId,
		Recursive:        request.Params.Recursive,
		SearchTerm:       request.Params.SearchTerm,
		SortBy:           request.Params.SortBy,
		SortOrder:        request.Params.SortOrder,
		StartIndex:       request.Params.StartIndex,
		Limit:            request.Params.Limit,
	})
	if err != nil {
		return nil, err
	}

	return api.GetTrailers200JSONResponse(result), nil
}
