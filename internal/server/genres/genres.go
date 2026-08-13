package genres

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/config"
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
		dtoList = append(dtoList, genreDto(genre))
	}

	return api.GetGenres200JSONResponse{
		Items:            &dtoList,
		StartIndex:       apiutil.Ptr(startIndex),
		TotalRecordCount: apiutil.Ptr(int32(total)),
	}, nil
}

func genreDto(genre items.Named) api.BaseItemDto {
	return api.BaseItemDto{
		Id:                &genre.ID,
		ServerId:          apiutil.Ptr(config.ServerID),
		Name:              apiutil.Ptr(genre.Name),
		SortName:          apiutil.Ptr(genre.Name),
		Type:              apiutil.Ptr(api.BaseItemKindGenre),
		IsFolder:          apiutil.Ptr(true),
		ImageTags:         &map[string]*string{},
		BackdropImageTags: &[]string{},
	}
}
