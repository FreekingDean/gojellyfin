package tvshows

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	serveritems "github.com/FreekingDean/gojellyfin/internal/server/items"
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
		int(apiutil.Deref(request.Params.Limit)),
	)
	if err != nil {
		return nil, err
	}

	episodes, err := serveritems.ItemDtos(ctx, s.items, records)
	if err != nil {
		return nil, err
	}

	return api.GetNextUp200JSONResponse{
		Items:            &episodes,
		StartIndex:       apiutil.Ptr(int32(0)),
		TotalRecordCount: apiutil.Ptr(int32(len(episodes))),
	}, nil
}
