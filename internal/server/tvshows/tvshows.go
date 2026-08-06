package tvshows

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/auth"
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

func (s *Server) GetNextUp(ctx context.Context, request api.GetNextUpRequestObject) (api.GetNextUpResponseObject, error) {
	records, err := s.items.NextUpEpisodes(ctx,
		auth.UserID(ctx),
		request.Params.SeriesId,
		int(dtos.Deref(request.Params.Limit)),
	)
	if err != nil {
		return nil, err
	}

	episodes, err := dtos.ItemDtos(ctx, s.items, records)
	if err != nil {
		return nil, err
	}

	return api.GetNextUp200JSONResponse{
		Items:            &episodes,
		StartIndex:       dtos.Ptr(int32(0)),
		TotalRecordCount: dtos.Ptr(int32(len(episodes))),
	}, nil
}
