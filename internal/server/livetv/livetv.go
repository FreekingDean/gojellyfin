package livetv

import (
	"context"
	"time"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

// There is no tuner and no guide, so the read paths answer an empty result
// rather than 501: a 501 makes the web client retry the section forever, while
// an empty result is both true and something it can render.
type Server struct{}

func New() *Server {
	return &Server{}
}

func noItems() api.BaseItemDtoQueryResult {
	return api.BaseItemDtoQueryResult{
		Items:            &[]api.BaseItemDto{},
		StartIndex:       apiutil.Ptr(int32(0)),
		TotalRecordCount: apiutil.Ptr(int32(0)),
	}
}

func (s *Server) GetLiveTvInfo(ctx context.Context, request api.GetLiveTvInfoRequestObject) (api.GetLiveTvInfoResponseObject, error) {
	return api.GetLiveTvInfo200JSONResponse{
		IsEnabled:    apiutil.Ptr(false),
		Services:     &[]api.LiveTvServiceInfo{},
		EnabledUsers: &[]string{},
	}, nil
}

func (s *Server) GetLiveTvChannels(ctx context.Context, request api.GetLiveTvChannelsRequestObject) (api.GetLiveTvChannelsResponseObject, error) {
	return api.GetLiveTvChannels200JSONResponse(noItems()), nil
}

func (s *Server) GetGuideInfo(ctx context.Context, request api.GetGuideInfoRequestObject) (api.GetGuideInfoResponseObject, error) {
	now := time.Now().UTC()
	return api.GetGuideInfo200JSONResponse{
		StartDate: apiutil.Ptr(now),
		EndDate:   apiutil.Ptr(now),
	}, nil
}

func (s *Server) GetLiveTvPrograms(ctx context.Context, request api.GetLiveTvProgramsRequestObject) (api.GetLiveTvProgramsResponseObject, error) {
	return api.GetLiveTvPrograms200JSONResponse(noItems()), nil
}

func (s *Server) GetPrograms(ctx context.Context, request api.GetProgramsRequestObject) (api.GetProgramsResponseObject, error) {
	return api.GetPrograms200JSONResponse(noItems()), nil
}

func (s *Server) GetRecommendedPrograms(ctx context.Context, request api.GetRecommendedProgramsRequestObject) (api.GetRecommendedProgramsResponseObject, error) {
	return api.GetRecommendedPrograms200JSONResponse(noItems()), nil
}

func (s *Server) GetRecordings(ctx context.Context, request api.GetRecordingsRequestObject) (api.GetRecordingsResponseObject, error) {
	return api.GetRecordings200JSONResponse(noItems()), nil
}

func (s *Server) GetRecordingFolders(ctx context.Context, request api.GetRecordingFoldersRequestObject) (api.GetRecordingFoldersResponseObject, error) {
	return api.GetRecordingFolders200JSONResponse(noItems()), nil
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
