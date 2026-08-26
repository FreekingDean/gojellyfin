package genres

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/server/dto"
)

type Server struct {
	items *items.Service
}

func New(items *items.Service) *Server {
	return &Server{items: items}
}

func (s *Server) GetGenres(ctx context.Context, request api.GetGenresRequestObject) (api.GetGenresResponseObject, error) {
	startIndex := apiutil.Deref(request.Params.StartIndex)

	named, total, err := s.items.DistinctGenres(ctx, items.MetadataQuery{
		LibraryID:  request.Params.ParentId,
		Kinds:      dto.Kinds(request.Params.IncludeItemTypes),
		SearchTerm: apiutil.Deref(request.Params.SearchTerm),
		StartIndex: int(startIndex),
		Limit:      int(apiutil.Deref(request.Params.Limit)),
	})
	if err != nil {
		return nil, err
	}

	dtoList := make([]api.BaseItemDto, 0, len(named))
	for _, genre := range named {
		dtoList = append(dtoList, dto.NamedItem(genre, api.BaseItemKindGenre, true))
	}

	return api.GetGenres200JSONResponse{
		Items:            &dtoList,
		StartIndex:       apiutil.Ptr(startIndex),
		TotalRecordCount: apiutil.Ptr(int32(total)),
	}, nil
}
