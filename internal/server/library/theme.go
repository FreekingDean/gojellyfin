package library

import (
	"context"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

// Theme songs and videos are extras sitting alongside the media. The scanner
// does not collect them, so every item reports none.
func (s *Server) GetThemeMedia(ctx context.Context, request api.GetThemeMediaRequestObject) (api.GetThemeMediaResponseObject, error) {
	return api.GetThemeMedia200JSONResponse{
		ThemeVideosResult:     apiutil.Ptr(themeMedia(request.ItemId)),
		ThemeSongsResult:      apiutil.Ptr(themeMedia(request.ItemId)),
		SoundtrackSongsResult: apiutil.Ptr(themeMedia(request.ItemId)),
	}, nil
}

func (s *Server) GetThemeSongs(ctx context.Context, request api.GetThemeSongsRequestObject) (api.GetThemeSongsResponseObject, error) {
	return api.GetThemeSongs200JSONResponse(themeMedia(request.ItemId)), nil
}

func (s *Server) GetThemeVideos(ctx context.Context, request api.GetThemeVideosRequestObject) (api.GetThemeVideosResponseObject, error) {
	return api.GetThemeVideos200JSONResponse(themeMedia(request.ItemId)), nil
}

func themeMedia(ownerID openapi_types.UUID) api.ThemeMediaResult {
	return api.ThemeMediaResult{
		OwnerId:          &ownerID,
		Items:            &[]api.BaseItemDto{},
		StartIndex:       apiutil.Ptr(int32(0)),
		TotalRecordCount: apiutil.Ptr(int32(0)),
	}
}
