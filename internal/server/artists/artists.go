package artists

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/server/dto"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
)

type Server struct {
	items     *items.Service
	libraries *libraries.Service
	policies  dto.Access
}

func New(items *items.Service, libraries *libraries.Service, policies dto.Access) *Server {
	return &Server{items: items, libraries: libraries, policies: policies}
}

func (s *Server) GetArtists(ctx context.Context, request api.GetArtistsRequestObject) (api.GetArtistsResponseObject, error) {
	result, err := s.artists(ctx, request.Params)
	if err != nil {
		return nil, err
	}

	return api.GetArtists200JSONResponse(result), nil
}

func (s *Server) GetAlbumArtists(ctx context.Context, request api.GetAlbumArtistsRequestObject) (api.GetAlbumArtistsResponseObject, error) {
	result, err := s.artists(ctx, api.GetArtistsParams(request.Params))
	if err != nil {
		return nil, err
	}

	return api.GetAlbumArtists200JSONResponse(result), nil
}

func (s *Server) GetArtistByName(ctx context.Context, request api.GetArtistByNameRequestObject) (api.GetArtistByNameResponseObject, error) {
	viewer, err := dto.ViewerFor(ctx, s.policies)
	if err != nil {
		return nil, err
	}

	artist, err := s.items.NamedItem(ctx, viewer, itemmodel.KindMusicArtist, request.Name)
	if err != nil {
		return nil, err
	}

	converted, err := dto.ItemDtos(ctx, s.items, []*items.Item{artist})
	if err != nil {
		return nil, err
	}

	return api.GetArtistByName200JSONResponse(converted[0]), nil
}

func (s *Server) artists(ctx context.Context, params api.GetArtistsParams) (api.BaseItemDtoQueryResult, error) {
	viewer, err := dto.ViewerFor(ctx, s.policies)
	if err != nil {
		return api.BaseItemDtoQueryResult{}, err
	}

	query := items.ItemQuery{
		Viewer:                  viewer,
		Kinds:                   []items.Kind{itemmodel.KindMusicArtist},
		SearchTerm:              apiutil.Deref(params.SearchTerm),
		StartIndex:              int(apiutil.Deref(params.StartIndex)),
		Limit:                   dto.Limit(params.Limit),
		Descending:              dto.Descending(params.SortOrder),
		SortBy:                  dto.SortFields(params.SortBy),
		NameStartsWith:          apiutil.Deref(params.NameStartsWith),
		NameStartsWithOrGreater: apiutil.Deref(params.NameStartsWithOrGreater),
		NameLessThan:            apiutil.Deref(params.NameLessThan),
	}
	if params.ParentId != nil {
		if library, err := s.libraries.Library(ctx, *params.ParentId); err == nil {
			query.LibraryID = &library.ID
		} else {
			query.ParentID = params.ParentId
		}
	}

	records, total, err := s.items.QueryItems(ctx, query)
	if err != nil {
		return api.BaseItemDtoQueryResult{}, err
	}

	converted, err := dto.ItemDtos(ctx, s.items, records)
	if err != nil {
		return api.BaseItemDtoQueryResult{}, err
	}

	return api.BaseItemDtoQueryResult{
		Items:            &converted,
		StartIndex:       apiutil.Ptr(int32(query.StartIndex)),
		TotalRecordCount: apiutil.Ptr(int32(total)),
	}, nil
}
