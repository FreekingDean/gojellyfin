package filter

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	serveritems "github.com/FreekingDean/gojellyfin/internal/server/items"
)

type Server struct {
	items *items.Service
}

func New(items *items.Service) *Server {
	return &Server{items: items}
}

func (s *Server) GetQueryFilters(ctx context.Context, request api.GetQueryFiltersRequestObject) (api.GetQueryFiltersResponseObject, error) {
	return api.GetQueryFilters200JSONResponse{
		Genres: &[]api.NameGuidPair{},
		Tags:   &[]string{},
	}, nil
}

// Only years are real: nothing extracts genres, tags or ratings yet, and the
// client needs the shape regardless.
func (s *Server) GetQueryFiltersLegacy(ctx context.Context, request api.GetQueryFiltersLegacyRequestObject) (api.GetQueryFiltersLegacyResponseObject, error) {
	years, err := s.items.DistinctYears(ctx, request.Params.ParentId, serveritems.Kinds(request.Params.IncludeItemTypes))
	if err != nil {
		return nil, err
	}

	return api.GetQueryFiltersLegacy200JSONResponse{
		Genres:          &[]string{},
		Tags:            &[]string{},
		OfficialRatings: &[]string{},
		Years:           &years,
	}, nil
}
