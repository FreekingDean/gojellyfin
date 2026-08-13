package filter

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/dto"
)

type Server struct {
	items *items.Service
}

func New(items *items.Service) *Server {
	return &Server{items: items}
}

func (s *Server) GetQueryFilters(ctx context.Context, request api.GetQueryFiltersRequestObject) (api.GetQueryFiltersResponseObject, error) {
	query := items.MetadataQuery{
		LibraryID: request.Params.ParentId,
		Kinds:     dto.Kinds(request.Params.IncludeItemTypes),
	}

	named, _, err := s.items.DistinctGenres(ctx, query)
	if err != nil {
		return nil, err
	}
	tags, err := s.items.DistinctTags(ctx, query)
	if err != nil {
		return nil, err
	}

	genres := make([]api.NameGuidPair, 0, len(named))
	for _, genre := range named {
		genres = append(genres, api.NameGuidPair{Id: &genre.ID, Name: &genre.Name})
	}

	return api.GetQueryFilters200JSONResponse{
		Genres: &genres,
		Tags:   &tags,
	}, nil
}

func (s *Server) GetQueryFiltersLegacy(ctx context.Context, request api.GetQueryFiltersLegacyRequestObject) (api.GetQueryFiltersLegacyResponseObject, error) {
	query := items.MetadataQuery{
		LibraryID: request.Params.ParentId,
		Kinds:     dto.Kinds(request.Params.IncludeItemTypes),
	}

	years, err := s.items.DistinctYears(ctx, request.Params.ParentId, query.Kinds)
	if err != nil {
		return nil, err
	}
	named, _, err := s.items.DistinctGenres(ctx, query)
	if err != nil {
		return nil, err
	}
	tags, err := s.items.DistinctTags(ctx, query)
	if err != nil {
		return nil, err
	}

	genres := make([]string, 0, len(named))
	for _, genre := range named {
		genres = append(genres, genre.Name)
	}

	return api.GetQueryFiltersLegacy200JSONResponse{
		Genres:          &genres,
		Tags:            &tags,
		OfficialRatings: &[]string{},
		Years:           &years,
	}, nil
}
