package library

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/server/dto"
)

func (s *Server) GetSimilarItems(ctx context.Context, request api.GetSimilarItemsRequestObject) (api.GetSimilarItemsResponseObject, error) {
	found, err := s.similar(ctx, request.ItemId, request.Params.Limit)
	if err != nil {
		return nil, err
	}

	return api.GetSimilarItems200JSONResponse(found), nil
}

func (s *Server) GetSimilarMovies(ctx context.Context, request api.GetSimilarMoviesRequestObject) (api.GetSimilarMoviesResponseObject, error) {
	found, err := s.similar(ctx, request.ItemId, request.Params.Limit)
	if err != nil {
		return nil, err
	}

	return api.GetSimilarMovies200JSONResponse(found), nil
}

func (s *Server) GetSimilarTrailers(ctx context.Context, request api.GetSimilarTrailersRequestObject) (api.GetSimilarTrailersResponseObject, error) {
	return api.GetSimilarTrailers200JSONResponse(dto.EmptyItems()), nil
}

func (s *Server) GetSimilarShows(ctx context.Context, request api.GetSimilarShowsRequestObject) (api.GetSimilarShowsResponseObject, error) {
	found, err := s.similar(ctx, request.ItemId, request.Params.Limit)
	if err != nil {
		return nil, err
	}

	return api.GetSimilarShows200JSONResponse(found), nil
}

func (s *Server) GetSimilarAlbums(ctx context.Context, request api.GetSimilarAlbumsRequestObject) (api.GetSimilarAlbumsResponseObject, error) {
	found, err := s.similar(ctx, request.ItemId, request.Params.Limit)
	if err != nil {
		return nil, err
	}

	return api.GetSimilarAlbums200JSONResponse(found), nil
}

func (s *Server) GetSimilarArtists(ctx context.Context, request api.GetSimilarArtistsRequestObject) (api.GetSimilarArtistsResponseObject, error) {
	found, err := s.similar(ctx, request.ItemId, request.Params.Limit)
	if err != nil {
		return nil, err
	}

	return api.GetSimilarArtists200JSONResponse(found), nil
}

func (s *Server) similar(ctx context.Context, itemID uuid.UUID, limit *int32) (api.BaseItemDtoQueryResult, error) {
	viewer, err := dto.ViewerFor(ctx, s.users)
	if err != nil {
		return api.BaseItemDtoQueryResult{}, err
	}

	records, err := s.items.Similar(ctx, viewer, itemID, int(apiutil.Deref(limit)))
	if errors.Is(err, items.ErrNotFound) {
		return dto.EmptyItems(), nil
	}
	if err != nil {
		return api.BaseItemDtoQueryResult{}, err
	}

	converted, err := dto.ItemDtos(ctx, s.items, records)
	if err != nil {
		return api.BaseItemDtoQueryResult{}, err
	}

	return api.BaseItemDtoQueryResult{
		Items:            &converted,
		StartIndex:       apiutil.Ptr(int32(0)),
		TotalRecordCount: apiutil.Ptr(int32(len(converted))),
	}, nil
}
