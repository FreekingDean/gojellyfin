package livetv

import (
	"cmp"
	"context"
	"errors"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/channels"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/server/dto"
)

const (
	guideDays      = 14
	defaultWindow  = 24 * time.Hour
	serviceName    = "gojellyfin"
	recommendLimit = 20
)

type Server struct {
	channels *channels.Service
	items    *items.Service
	policies dto.Access
}

func New(channels *channels.Service, items *items.Service, policies dto.Access) *Server {
	return &Server{channels: channels, items: items, policies: policies}
}

func (s *Server) GetLiveTvInfo(ctx context.Context, request api.GetLiveTvInfoRequestObject) (api.GetLiveTvInfoResponseObject, error) {
	viewer, err := dto.ViewerFor(ctx, s.policies)
	if err != nil {
		return nil, err
	}
	listed, err := s.channels.List(ctx, viewer)
	if err != nil {
		return nil, err
	}

	enabled := []string{}
	if user := auth.UserID(ctx); user != uuid.Nil {
		enabled = append(enabled, user.String())
	}

	return api.GetLiveTvInfo200JSONResponse{
		IsEnabled: apiutil.Ptr(len(listed) > 0),
		Services: &[]api.LiveTvServiceInfo{{
			Name:               apiutil.Ptr(serviceName),
			Status:             apiutil.Ptr(api.LiveTvServiceStatusOk),
			IsVisible:          apiutil.Ptr(false),
			HasUpdateAvailable: apiutil.Ptr(false),
			Tuners:             &[]string{},
		}},
		EnabledUsers: &enabled,
	}, nil
}

func (s *Server) GetLiveTvChannels(ctx context.Context, request api.GetLiveTvChannelsRequestObject) (api.GetLiveTvChannelsResponseObject, error) {
	params := request.Params
	viewer, err := dto.ViewerFor(ctx, s.policies)
	if err != nil {
		return nil, err
	}

	listed := []*channels.Channel{}
	if !apiutil.OrElse(params.IsFavorite, false) && apiutil.OrElse(params.Type, api.TV) == api.TV {
		if listed, err = s.channels.List(ctx, viewer); err != nil {
			return nil, err
		}
	}

	total := len(listed)
	listed = page(listed, params.StartIndex, params.Limit)

	airing := map[uuid.UUID]channels.Program{}
	if apiutil.OrElse(params.AddCurrentProgram, true) {
		if airing, err = s.airing(ctx, listed); err != nil {
			return nil, err
		}
	}

	converted, err := dto.ChannelDtos(ctx, s.items, listed, airing)
	if err != nil {
		return nil, err
	}

	return api.GetLiveTvChannels200JSONResponse{
		Items:            &converted,
		StartIndex:       apiutil.Ptr(apiutil.OrElse(params.StartIndex, 0)),
		TotalRecordCount: apiutil.Ptr(int32(total)),
	}, nil
}

func (s *Server) GetChannel(ctx context.Context, request api.GetChannelRequestObject) (api.GetChannelResponseObject, error) {
	viewer, err := dto.ViewerFor(ctx, s.policies)
	if err != nil {
		return nil, err
	}

	channel, err := s.channels.Channel(ctx, viewer, request.ChannelId)
	if errors.Is(err, channels.ErrNotFound) {
		return api.GetChannel404JSONResponse{}, nil
	}
	if err != nil {
		return nil, err
	}

	airing, err := s.airing(ctx, []*channels.Channel{channel})
	if err != nil {
		return nil, err
	}
	converted, err := dto.ChannelDtos(ctx, s.items, []*channels.Channel{channel}, airing)
	if err != nil {
		return nil, err
	}

	return api.GetChannel200JSONResponse(converted[0]), nil
}

func (s *Server) airing(ctx context.Context, listed []*channels.Channel) (map[uuid.UUID]channels.Program, error) {
	now := s.channels.Now()
	airing := make(map[uuid.UUID]channels.Program, len(listed))
	for _, channel := range listed {
		schedule, err := s.channels.Schedule(ctx, channel)
		if err != nil {
			return nil, err
		}
		if current, found := schedule.At(now); found {
			airing[channel.ID] = channels.Program{Channel: channel, Airing: current}
		}
	}

	return airing, nil
}

func (s *Server) GetGuideInfo(ctx context.Context, request api.GetGuideInfoRequestObject) (api.GetGuideInfoResponseObject, error) {
	now := s.channels.Now().UTC()

	return api.GetGuideInfo200JSONResponse{
		StartDate: apiutil.Ptr(now),
		EndDate:   apiutil.Ptr(now.AddDate(0, 0, guideDays)),
	}, nil
}

func (s *Server) GetLiveTvPrograms(ctx context.Context, request api.GetLiveTvProgramsRequestObject) (api.GetLiveTvProgramsResponseObject, error) {
	params := request.Params
	result, err := s.programs(ctx, filter{
		channelIDs:   apiutil.Deref(params.ChannelIds),
		minStartDate: params.MinStartDate,
		maxStartDate: params.MaxStartDate,
		minEndDate:   params.MinEndDate,
		maxEndDate:   params.MaxEndDate,
		hasAired:     params.HasAired,
		isAiring:     params.IsAiring,
		isMovie:      params.IsMovie,
		isSeries:     params.IsSeries,
		startIndex:   params.StartIndex,
		limit:        params.Limit,
	})
	if err != nil {
		return nil, err
	}

	return api.GetLiveTvPrograms200JSONResponse(result), nil
}

func (s *Server) GetPrograms(ctx context.Context, request api.GetProgramsRequestObject) (api.GetProgramsResponseObject, error) {
	body := apiutil.Body(request.JSONBody, request.ApplicationWildcardPlusJSONBody)
	if body == nil {
		body = &api.GetProgramsDto{}
	}

	result, err := s.programs(ctx, filter{
		channelIDs:   apiutil.Deref(body.ChannelIds),
		minStartDate: body.MinStartDate,
		maxStartDate: body.MaxStartDate,
		minEndDate:   body.MinEndDate,
		maxEndDate:   body.MaxEndDate,
		hasAired:     body.HasAired,
		isAiring:     body.IsAiring,
		isMovie:      body.IsMovie,
		isSeries:     body.IsSeries,
		startIndex:   body.StartIndex,
		limit:        body.Limit,
	})
	if err != nil {
		return nil, err
	}

	return api.GetPrograms200JSONResponse(result), nil
}

func (s *Server) GetRecommendedPrograms(ctx context.Context, request api.GetRecommendedProgramsRequestObject) (api.GetRecommendedProgramsResponseObject, error) {
	params := request.Params
	result, err := s.programs(ctx, filter{
		hasAired:   params.HasAired,
		isAiring:   params.IsAiring,
		isMovie:    params.IsMovie,
		isSeries:   params.IsSeries,
		startIndex: params.StartIndex,
		limit:      apiutil.Ptr(apiutil.OrElse(params.Limit, recommendLimit)),
	})
	if err != nil {
		return nil, err
	}

	return api.GetRecommendedPrograms200JSONResponse(result), nil
}

func (s *Server) GetProgram(ctx context.Context, request api.GetProgramRequestObject) (api.GetProgramResponseObject, error) {
	id, err := uuid.Parse(request.ProgramId)
	if err != nil {
		return api.GetProgram404JSONResponse{}, nil
	}

	viewer, err := dto.ViewerFor(ctx, s.policies)
	if err != nil {
		return nil, err
	}

	program, err := s.channels.Program(ctx, viewer, id)
	if errors.Is(err, channels.ErrNotFound) {
		return api.GetProgram404JSONResponse{}, nil
	}
	if err != nil {
		return nil, err
	}

	converted, err := dto.ProgramDtos(ctx, s.items, []channels.Program{program})
	if err != nil {
		return nil, err
	}

	return api.GetProgram200JSONResponse(converted[0]), nil
}

type filter struct {
	channelIDs   []uuid.UUID
	minStartDate *time.Time
	maxStartDate *time.Time
	minEndDate   *time.Time
	maxEndDate   *time.Time
	hasAired     *bool
	isAiring     *bool
	isMovie      *bool
	isSeries     *bool
	startIndex   *int32
	limit        *int32
}

func (f filter) window(now time.Time) (time.Time, time.Time) {
	from := now
	switch {
	case f.minEndDate != nil:
		from = *f.minEndDate
	case f.minStartDate != nil:
		from = *f.minStartDate
	case apiutil.OrElse(f.hasAired, false):
		from = now.Add(-defaultWindow)
	}

	to := from.Add(defaultWindow)
	switch {
	case f.maxStartDate != nil:
		to = f.maxStartDate.Add(time.Nanosecond)
	case f.maxEndDate != nil:
		to = *f.maxEndDate
	case apiutil.OrElse(f.isAiring, false):
		to = now.Add(time.Nanosecond)
	case apiutil.OrElse(f.hasAired, false):
		to = now
	}

	if earliest := now.AddDate(0, 0, -guideDays); from.Before(earliest) {
		from = earliest
	}
	if latest := from.AddDate(0, 0, guideDays); to.After(latest) {
		to = latest
	}

	return from, to
}

func (f filter) keeps(program channels.Program, now time.Time) bool {
	airing := !program.Start.After(now) && program.End.After(now)
	aired := !program.End.After(now)

	return (f.minStartDate == nil || !program.Start.Before(*f.minStartDate)) &&
		(f.maxStartDate == nil || !program.Start.After(*f.maxStartDate)) &&
		(f.minEndDate == nil || !program.End.Before(*f.minEndDate)) &&
		(f.maxEndDate == nil || !program.End.After(*f.maxEndDate)) &&
		(f.isAiring == nil || *f.isAiring == airing) &&
		(f.hasAired == nil || *f.hasAired == aired)
}

func (s *Server) programs(ctx context.Context, f filter) (api.BaseItemDtoQueryResult, error) {
	viewer, err := dto.ViewerFor(ctx, s.policies)
	if err != nil {
		return api.BaseItemDtoQueryResult{}, err
	}

	now := s.channels.Now()
	from, to := f.window(now)
	listed, err := s.channels.Programs(ctx, channels.Guide{Viewer: viewer, ChannelIDs: f.channelIDs, From: from, To: to})
	if err != nil {
		return api.BaseItemDtoQueryResult{}, err
	}

	kept := []channels.Program{}
	for _, program := range listed {
		if f.keeps(program, now) {
			kept = append(kept, program)
		}
	}
	slices.SortStableFunc(kept, func(a, b channels.Program) int {
		return cmp.Or(a.Start.Compare(b.Start), cmp.Compare(a.Channel.Number, b.Channel.Number))
	})

	converted, err := dto.ProgramDtos(ctx, s.items, kept)
	if err != nil {
		return api.BaseItemDtoQueryResult{}, err
	}
	converted = slices.DeleteFunc(converted, func(program api.BaseItemDto) bool {
		return (f.isMovie != nil && apiutil.Deref(program.IsMovie) != *f.isMovie) ||
			(f.isSeries != nil && apiutil.Deref(program.IsSeries) != *f.isSeries)
	})

	total := len(converted)
	converted = page(converted, f.startIndex, f.limit)

	return api.BaseItemDtoQueryResult{
		Items:            &converted,
		StartIndex:       apiutil.Ptr(apiutil.OrElse(f.startIndex, 0)),
		TotalRecordCount: apiutil.Ptr(int32(total)),
	}, nil
}

func page[T any](listed []T, startIndex, limit *int32) []T {
	start := min(int(apiutil.OrElse(startIndex, 0)), len(listed))
	listed = listed[max(start, 0):]
	if count := int(apiutil.OrElse(limit, 0)); count > 0 && count < len(listed) {
		listed = listed[:count]
	}

	return listed
}

func (s *Server) GetRecordings(ctx context.Context, request api.GetRecordingsRequestObject) (api.GetRecordingsResponseObject, error) {
	return api.GetRecordings200JSONResponse(dto.EmptyItems()), nil
}

func (s *Server) GetRecordingFolders(ctx context.Context, request api.GetRecordingFoldersRequestObject) (api.GetRecordingFoldersResponseObject, error) {
	return api.GetRecordingFolders200JSONResponse(dto.EmptyItems()), nil
}

func (s *Server) GetTimers(ctx context.Context, request api.GetTimersRequestObject) (api.GetTimersResponseObject, error) {
	return api.GetTimers200JSONResponse{
		Items:            &[]api.TimerInfoDto{},
		StartIndex:       apiutil.Ptr(int32(0)),
		TotalRecordCount: apiutil.Ptr(int32(0)),
	}, nil
}

func (s *Server) GetSeriesTimers(ctx context.Context, request api.GetSeriesTimersRequestObject) (api.GetSeriesTimersResponseObject, error) {
	return api.GetSeriesTimers200JSONResponse{
		Items:            &[]api.SeriesTimerInfoDto{},
		StartIndex:       apiutil.Ptr(int32(0)),
		TotalRecordCount: apiutil.Ptr(int32(0)),
	}, nil
}

func (s *Server) GetTunerHostTypes(ctx context.Context, request api.GetTunerHostTypesRequestObject) (api.GetTunerHostTypesResponseObject, error) {
	return api.GetTunerHostTypes200JSONResponse{}, nil
}
