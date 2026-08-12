package years

import (
	"context"
	"strconv"

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

func (s *Server) GetYears(ctx context.Context, request api.GetYearsRequestObject) (api.GetYearsResponseObject, error) {
	years, err := s.items.DistinctYears(ctx, request.Params.ParentId, dto.Kinds(request.Params.IncludeItemTypes))
	if err != nil {
		return nil, err
	}

	dtoList := make([]api.BaseItemDto, 0, len(years))
	for _, year := range years {
		dtoList = append(dtoList, yearDto(year))
	}

	return api.GetYears200JSONResponse{
		Items:            &dtoList,
		StartIndex:       apiutil.Ptr(int32(0)),
		TotalRecordCount: apiutil.Ptr(int32(len(dtoList))),
	}, nil
}

func (s *Server) GetYear(ctx context.Context, request api.GetYearRequestObject) (api.GetYearResponseObject, error) {
	return api.GetYear200JSONResponse(yearDto(request.Year)), nil
}

func yearDto(year int32) api.BaseItemDto {
	name := strconv.Itoa(int(year))

	return api.BaseItemDto{
		Name:              apiutil.Ptr(name),
		SortName:          apiutil.Ptr(name),
		ServerId:          apiutil.Ptr(config.ServerID),
		Type:              apiutil.Ptr(api.BaseItemKindYear),
		ProductionYear:    apiutil.Ptr(year),
		IsFolder:          apiutil.Ptr(true),
		ImageTags:         &map[string]*string{},
		BackdropImageTags: &[]string{},
	}
}
