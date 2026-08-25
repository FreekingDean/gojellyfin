package items

import (
	"context"

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

func (s *Server) GetItems(ctx context.Context, request api.GetItemsRequestObject) (api.GetItemsResponseObject, error) {
	result, err := dto.QueryResult(ctx, s.items, s.libraries, request.Params)
	if err != nil {
		return nil, err
	}

	return api.GetItems200JSONResponse(result), nil
}

func (s *Server) GetItem(ctx context.Context, request api.GetItemRequestObject) (api.GetItemResponseObject, error) {
	item, err := s.items.ItemByID(ctx, request.ItemId)
	if err != nil {
		// Clients navigate into a library by asking for it as an item, but
		// libraries are rows in their own table rather than items.
		library, libraryErr := s.libraries.Library(ctx, request.ItemId)
		if libraryErr != nil {
			return api.GetItem403Response{}, nil
		}

		return api.GetItem200JSONResponse(dto.LibraryView(library)), nil
	}

	converted, err := dto.ItemDtos(ctx, s.items, []*items.Item{item})
	if err != nil {
		return nil, err
	}

	return api.GetItem200JSONResponse(converted[0]), nil
}

func (s *Server) GetRootFolder(ctx context.Context, request api.GetRootFolderRequestObject) (api.GetRootFolderResponseObject, error) {
	return api.GetRootFolder200JSONResponse(dto.RootView()), nil
}

func (s *Server) GetLatestMedia(ctx context.Context, request api.GetLatestMediaRequestObject) (api.GetLatestMediaResponseObject, error) {
	query := items.ItemQuery{
		Kinds:      []items.Kind{itemmodal.KindMovie, itemmodal.KindSeries},
		SortBy:     []string{"DateCreated"},
		Descending: true,
		Limit:      int(apiutil.OrElse(request.Params.Limit, int32(20))),
	}
	if request.Params.ParentId != nil {
		query.LibraryID = request.Params.ParentId
	}

	records, _, err := s.items.QueryItems(ctx, query)
	if err != nil {
		return nil, err
	}

	converted, err := dto.ItemDtos(ctx, s.items, records)
	if err != nil {
		return nil, err
	}

	return api.GetLatestMedia200JSONResponse(converted), nil
}
