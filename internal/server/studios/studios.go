package studios

import (
	"context"

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

func (s *Server) GetStudios(ctx context.Context, request api.GetStudiosRequestObject) (api.GetStudiosResponseObject, error) {
	viewer, err := dto.ViewerFor(ctx, s.policies)
	if err != nil {
		return nil, err
	}

	startIndex := apiutil.Deref(request.Params.StartIndex)

	named, total, err := s.items.DistinctStudios(ctx, items.MetadataQuery{
		Viewer:     viewer,
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
	for _, studio := range named {
		dtoList = append(dtoList, dto.NamedItem(studio, api.BaseItemKindStudio, true))
	}

	return api.GetStudios200JSONResponse{
		Items:            &dtoList,
		StartIndex:       apiutil.Ptr(startIndex),
		TotalRecordCount: apiutil.Ptr(int32(total)),
	}, nil
}

func (s *Server) GetStudio(ctx context.Context, request api.GetStudioRequestObject) (api.GetStudioResponseObject, error) {
	viewer, err := dto.ViewerFor(ctx, s.policies)
	if err != nil {
		return nil, err
	}

	studio, err := s.items.StudioNamed(ctx, items.MetadataQuery{Viewer: viewer}, request.Name)
	if err != nil {
		return nil, err
	}

	return api.GetStudio200JSONResponse(dto.NamedItem(studio, api.BaseItemKindStudio, true)), nil
}
