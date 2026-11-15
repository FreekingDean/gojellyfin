package instantmix

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/server/dto"
)

type Server struct {
	items    *items.Service
	policies dto.Access
}

func New(items *items.Service, policies dto.Access) *Server {
	return &Server{items: items, policies: policies}
}

func (s *Server) GetInstantMixFromAlbum(ctx context.Context, request api.GetInstantMixFromAlbumRequestObject) (api.GetInstantMixFromAlbumResponseObject, error) {
	mix, err := s.fromItem(ctx, request.ItemId, request.Params.Limit)
	if err != nil {
		return nil, err
	}

	return api.GetInstantMixFromAlbum200JSONResponse(mix), nil
}

func (s *Server) GetInstantMixFromArtists(ctx context.Context, request api.GetInstantMixFromArtistsRequestObject) (api.GetInstantMixFromArtistsResponseObject, error) {
	mix, err := s.fromItem(ctx, request.ItemId, request.Params.Limit)
	if err != nil {
		return nil, err
	}

	return api.GetInstantMixFromArtists200JSONResponse(mix), nil
}

func (s *Server) GetInstantMixFromItem(ctx context.Context, request api.GetInstantMixFromItemRequestObject) (api.GetInstantMixFromItemResponseObject, error) {
	mix, err := s.fromItem(ctx, request.ItemId, request.Params.Limit)
	if err != nil {
		return nil, err
	}

	return api.GetInstantMixFromItem200JSONResponse(mix), nil
}

func (s *Server) GetInstantMixFromPlaylist(ctx context.Context, request api.GetInstantMixFromPlaylistRequestObject) (api.GetInstantMixFromPlaylistResponseObject, error) {
	mix, err := s.fromItem(ctx, request.ItemId, request.Params.Limit)
	if err != nil {
		return nil, err
	}

	return api.GetInstantMixFromPlaylist200JSONResponse(mix), nil
}

func (s *Server) GetInstantMixFromSong(ctx context.Context, request api.GetInstantMixFromSongRequestObject) (api.GetInstantMixFromSongResponseObject, error) {
	mix, err := s.fromItem(ctx, request.ItemId, request.Params.Limit)
	if err != nil {
		return nil, err
	}

	return api.GetInstantMixFromSong200JSONResponse(mix), nil
}

func (s *Server) GetInstantMixFromMusicGenreById(ctx context.Context, request api.GetInstantMixFromMusicGenreByIdRequestObject) (api.GetInstantMixFromMusicGenreByIdResponseObject, error) {
	mix, err := s.fromGenre(ctx, &request.Params.Id, "", request.Params.Limit)
	if err != nil {
		return nil, err
	}

	return api.GetInstantMixFromMusicGenreById200JSONResponse(mix), nil
}

func (s *Server) GetInstantMixFromMusicGenreByName(ctx context.Context, request api.GetInstantMixFromMusicGenreByNameRequestObject) (api.GetInstantMixFromMusicGenreByNameResponseObject, error) {
	mix, err := s.fromGenre(ctx, nil, request.Name, request.Params.Limit)
	if err != nil {
		return nil, err
	}

	return api.GetInstantMixFromMusicGenreByName200JSONResponse(mix), nil
}

func (s *Server) fromItem(ctx context.Context, itemID uuid.UUID, limit *int32) (api.BaseItemDtoQueryResult, error) {
	viewer, err := dto.ViewerFor(ctx, s.policies)
	if err != nil {
		return api.BaseItemDtoQueryResult{}, err
	}

	tracks, err := s.items.Mix(ctx, viewer, itemID, int(apiutil.Deref(limit)))
	if errors.Is(err, items.ErrNotFound) {
		return dto.EmptyItems(), nil
	}
	if err != nil {
		return api.BaseItemDtoQueryResult{}, err
	}

	return s.result(ctx, tracks)
}

func (s *Server) fromGenre(ctx context.Context, genreID *uuid.UUID, name string, limit *int32) (api.BaseItemDtoQueryResult, error) {
	viewer, err := dto.ViewerFor(ctx, s.policies)
	if err != nil {
		return api.BaseItemDtoQueryResult{}, err
	}

	tracks, err := s.items.GenreMix(ctx, viewer, genreID, name, int(apiutil.Deref(limit)))
	if err != nil {
		return api.BaseItemDtoQueryResult{}, err
	}

	return s.result(ctx, tracks)
}

func (s *Server) result(ctx context.Context, tracks []*items.Item) (api.BaseItemDtoQueryResult, error) {
	converted, err := dto.ItemDtos(ctx, s.items, tracks)
	if err != nil {
		return api.BaseItemDtoQueryResult{}, err
	}

	return api.BaseItemDtoQueryResult{
		Items:            &converted,
		StartIndex:       apiutil.Ptr(int32(0)),
		TotalRecordCount: apiutil.Ptr(int32(len(converted))),
	}, nil
}
