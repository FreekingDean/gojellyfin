package persons

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

func (s *Server) GetPersons(ctx context.Context, request api.GetPersonsRequestObject) (api.GetPersonsResponseObject, error) {
	named, total, err := s.items.DistinctPeople(ctx, items.MetadataQuery{
		ItemID:     request.Params.AppearsInItemId,
		SearchTerm: apiutil.Deref(request.Params.SearchTerm),
		Limit:      int(apiutil.Deref(request.Params.Limit)),
	}, dto.Accepted(request.Params.PersonTypes, items.ValidCreditKind))
	if err != nil {
		return nil, err
	}

	dtoList := make([]api.BaseItemDto, 0, len(named))
	for _, person := range named {
		dtoList = append(dtoList, personDto(person))
	}

	return api.GetPersons200JSONResponse{
		Items:            &dtoList,
		StartIndex:       apiutil.Ptr(int32(0)),
		TotalRecordCount: apiutil.Ptr(int32(total)),
	}, nil
}

func personDto(person items.Named) api.BaseItemDto {
	return api.BaseItemDto{
		Id:                &person.ID,
		ServerId:          apiutil.Ptr(config.ServerID),
		Name:              apiutil.Ptr(person.Name),
		SortName:          apiutil.Ptr(person.Name),
		Type:              apiutil.Ptr(api.BaseItemKindPerson),
		IsFolder:          apiutil.Ptr(false),
		ImageTags:         &map[string]*string{},
		BackdropImageTags: &[]string{},
	}
}
