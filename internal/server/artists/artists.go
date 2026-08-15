package artists

import (
	"context"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/server/dto"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
)

type Server struct {
	items     *items.Service
	libraries *libraries.Service
}

func New(items *items.Service, libraries *libraries.Service) *Server {
	return &Server{items: items, libraries: libraries}
}

type scope struct {
	parentID   *uuid.UUID
	searchTerm *string
	startIndex *int32
	limit      *int32
	sortOrder  *[]api.SortOrder
	childKinds []items.Kind
}

func (s *Server) GetArtists(ctx context.Context, request api.GetArtistsRequestObject) (api.GetArtistsResponseObject, error) {
	result, err := s.query(ctx, scope{
		parentID:   request.Params.ParentId,
		searchTerm: request.Params.SearchTerm,
		startIndex: request.Params.StartIndex,
		limit:      request.Params.Limit,
		sortOrder:  request.Params.SortOrder,
	})
	if err != nil {
		return nil, err
	}

	return api.GetArtists200JSONResponse(result), nil
}

// An album artist is an artist a scanned album sits under, which is the whole
// of the distinction a folder layout can carry.
func (s *Server) GetAlbumArtists(ctx context.Context, request api.GetAlbumArtistsRequestObject) (api.GetAlbumArtistsResponseObject, error) {
	result, err := s.query(ctx, scope{
		parentID:   request.Params.ParentId,
		searchTerm: request.Params.SearchTerm,
		startIndex: request.Params.StartIndex,
		limit:      request.Params.Limit,
		sortOrder:  request.Params.SortOrder,
		childKinds: []items.Kind{itemmodal.KindMusicAlbum},
	})
	if err != nil {
		return nil, err
	}

	return api.GetAlbumArtists200JSONResponse(result), nil
}

func (s *Server) GetArtistByName(ctx context.Context, request api.GetArtistByNameRequestObject) (api.GetArtistByNameResponseObject, error) {
	artist, err := s.items.ItemByName(ctx, itemmodal.KindMusicArtist, request.Name)
	if err != nil {
		return api.GetArtistByName403Response{}, nil
	}

	converted, err := dto.ItemDtos(ctx, s.items, []*items.Item{artist})
	if err != nil {
		return nil, err
	}

	return api.GetArtistByName200JSONResponse(converted[0]), nil
}

func (s *Server) query(ctx context.Context, requested scope) (api.BaseItemDtoQueryResult, error) {
	query := items.ItemQuery{
		Kinds:      []items.Kind{itemmodal.KindMusicArtist},
		ChildKinds: requested.childKinds,
		SearchTerm: apiutil.Deref(requested.searchTerm),
		StartIndex: int(apiutil.Deref(requested.startIndex)),
		Limit:      int(apiutil.Deref(requested.limit)),
		Descending: dto.Descending(requested.sortOrder),
	}
	dto.ScopeToParent(ctx, s.libraries, &query, requested.parentID, true)

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
