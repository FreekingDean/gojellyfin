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

func (s *Server) GetSeasons(ctx context.Context, request api.GetSeasonsRequestObject) (api.GetSeasonsResponseObject, error) {
	records, err := s.items.SeriesSeasons(ctx, request.SeriesId)
	if err != nil {
		return nil, err
	}

	seasons, err := serveritems.ItemDtos(ctx, s.items, records)
	if err != nil {
		return nil, err
	}

	return api.GetSeasons200JSONResponse{
		Items:            &seasons,
		StartIndex:       apiutil.Ptr(int32(0)),
		TotalRecordCount: apiutil.Ptr(int32(len(seasons))),
	}, nil
}

func (s *Server) GetEpisodes(ctx context.Context, request api.GetEpisodesRequestObject) (api.GetEpisodesResponseObject, error) {
	startIndex := int(apiutil.Deref(request.Params.StartIndex))
	records, total, err := s.items.SeriesEpisodes(ctx, items.EpisodeQuery{
		SeriesID:   request.SeriesId,
		SeasonID:   request.Params.SeasonId,
		Season:     request.Params.Season,
		StartIndex: startIndex,
		Limit:      int(apiutil.Deref(request.Params.Limit)),
	})
	if err != nil {
		return nil, err
	}

	episodes, err := serveritems.ItemDtos(ctx, s.items, records)
	if err != nil {
		return nil, err
	}

	return api.GetEpisodes200JSONResponse{
		Items:            &episodes,
		StartIndex:       apiutil.Ptr(int32(startIndex)),
		TotalRecordCount: apiutil.Ptr(int32(total)),
	}, nil
}

func (s *Server) GetUpcomingEpisodes(ctx context.Context, request api.GetUpcomingEpisodesRequestObject) (api.GetUpcomingEpisodesResponseObject, error) {
	startIndex := int(apiutil.Deref(request.Params.StartIndex))
	records, total, err := s.items.UpcomingEpisodes(ctx,
		request.Params.ParentId,
		startIndex,
		int(apiutil.Deref(request.Params.Limit)),
	)
	if err != nil {
		return nil, err
	}

	episodes, err := serveritems.ItemDtos(ctx, s.items, records)
	if err != nil {
		return nil, err
	}

	return api.GetUpcomingEpisodes200JSONResponse{
		Items:            &episodes,
		StartIndex:       apiutil.Ptr(int32(startIndex)),
		TotalRecordCount: apiutil.Ptr(int32(total)),
	}, nil
}
