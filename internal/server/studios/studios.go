package studios

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/config"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

type Server struct {
	items *items.Service
}

func New(items *items.Service) *Server {
	return &Server{items: items}
}

func (s *Server) GetStudios(ctx context.Context, request api.GetStudiosRequestObject) (api.GetStudiosResponseObject, error) {
	startIndex := apiutil.Deref(request.Params.StartIndex)

	named, total, err := s.items.DistinctStudios(ctx, items.MetadataQuery{
		LibraryID:  request.Params.ParentId,
		Kinds:      kinds(request.Params.IncludeItemTypes),
		SearchTerm: apiutil.Deref(request.Params.SearchTerm),
		StartIndex: int(startIndex),
		Limit:      int(apiutil.Deref(request.Params.Limit)),
	})
	if err != nil {
		return nil, err
	}

	dtoList := make([]api.BaseItemDto, 0, len(named))
	for _, studio := range named {
		dtoList = append(dtoList, studioDto(studio))
	}

	return api.GetStudios200JSONResponse{
		Items:            &dtoList,
		StartIndex:       apiutil.Ptr(startIndex),
		TotalRecordCount: apiutil.Ptr(int32(total)),
	}, nil
}

func studioDto(studio items.Named) api.BaseItemDto {
	return api.BaseItemDto{
		Id:                &studio.ID,
		ServerId:          apiutil.Ptr(config.ServerID),
		Name:              apiutil.Ptr(studio.Name),
		SortName:          apiutil.Ptr(studio.Name),
		Type:              apiutil.Ptr(api.BaseItemKindStudio),
		IsFolder:          apiutil.Ptr(true),
		ImageTags:         &map[string]*string{},
		BackdropImageTags: &[]string{},
	}
}

func kinds(types *[]api.BaseItemKind) []items.Kind {
	if types == nil {
		return nil
	}

	valid := make([]items.Kind, 0, len(*types))
	for _, value := range *types {
		kind := items.Kind(value)
		if items.ValidKind(kind) == nil {
			valid = append(valid, kind)
		}
	}

	return valid
}
