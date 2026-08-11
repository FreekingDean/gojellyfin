package trailers

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

func (s *Server) GetTrailers(ctx context.Context, request api.GetTrailersRequestObject) (api.GetTrailersResponseObject, error) {
	result, err := s.items.QueryResult(ctx, api.GetItemsParams{
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
