package years

import (
	"context"
	"strconv"

	"github.com/FreekingDean/gojellyfin/internal/config"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/dtos"
)

type Server struct {
	items *items.Service
}

func New(items *items.Service) *Server {
	return &Server{items: items}
}

func (s *Server) GetYears(ctx context.Context, request api.GetYearsRequestObject) (api.GetYearsResponseObject, error) {
	years, err := s.items.DistinctYears(ctx, request.Params.ParentId, itemTypes(request.Params.IncludeItemTypes))
	if err != nil {
		return nil, err
	}

	dtoList := make([]api.BaseItemDto, 0, len(years))
	for _, year := range years {
		dtoList = append(dtoList, yearDto(year))
	}

	return api.GetYears200JSONResponse{
		Items:            &dtoList,
		StartIndex:       dtos.Ptr(int32(0)),
		TotalRecordCount: dtos.Ptr(int32(len(dtoList))),
	}, nil
}

func (s *Server) GetYear(ctx context.Context, request api.GetYearRequestObject) (api.GetYearResponseObject, error) {
	return api.GetYear200JSONResponse(yearDto(request.Year)), nil
}

func yearDto(year int32) api.BaseItemDto {
	name := strconv.Itoa(int(year))

	return api.BaseItemDto{
		Name:              dtos.Ptr(name),
		SortName:          dtos.Ptr(name),
		ServerId:          dtos.Ptr(config.ServerID),
		Type:              dtos.Ptr(api.BaseItemKindYear),
		ProductionYear:    dtos.Ptr(year),
		IsFolder:          dtos.Ptr(true),
		ImageTags:         &map[string]string{},
		BackdropImageTags: &[]string{},
	}
}

func itemTypes(kinds *[]api.BaseItemKind) []string {
	if kinds == nil {
		return nil
	}

	types := make([]string, 0, len(*kinds))
	for _, kind := range *kinds {
		types = append(types, string(kind))
	}

	return types
}
