package library

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

func (s *Server) GetSimilarItems(ctx context.Context, request api.GetSimilarItemsRequestObject) (api.GetSimilarItemsResponseObject, error) {
	return api.GetSimilarItems200JSONResponse(noSimilarItems()), nil
}

func (s *Server) GetSimilarMovies(ctx context.Context, request api.GetSimilarMoviesRequestObject) (api.GetSimilarMoviesResponseObject, error) {
	return api.GetSimilarMovies200JSONResponse(noSimilarItems()), nil
}

func (s *Server) GetSimilarTrailers(ctx context.Context, request api.GetSimilarTrailersRequestObject) (api.GetSimilarTrailersResponseObject, error) {
	return api.GetSimilarTrailers200JSONResponse(noSimilarItems()), nil
}

func (s *Server) GetSimilarShows(ctx context.Context, request api.GetSimilarShowsRequestObject) (api.GetSimilarShowsResponseObject, error) {
	return api.GetSimilarShows200JSONResponse(noSimilarItems()), nil
}

func (s *Server) GetSimilarAlbums(ctx context.Context, request api.GetSimilarAlbumsRequestObject) (api.GetSimilarAlbumsResponseObject, error) {
	return api.GetSimilarAlbums200JSONResponse(noSimilarItems()), nil
}

func (s *Server) GetSimilarArtists(ctx context.Context, request api.GetSimilarArtistsRequestObject) (api.GetSimilarArtistsResponseObject, error) {
	return api.GetSimilarArtists200JSONResponse(noSimilarItems()), nil
}

func noSimilarItems() api.BaseItemDtoQueryResult {
	return api.BaseItemDtoQueryResult{
		Items:            &[]api.BaseItemDto{},
		StartIndex:       apiutil.Ptr(int32(0)),
		TotalRecordCount: apiutil.Ptr(int32(0)),
	}
}
