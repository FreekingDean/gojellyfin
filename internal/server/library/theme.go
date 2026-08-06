package library

import (
	"context"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/dtos"
)

// Theme songs and videos are extras sitting alongside the media. The scanner
// does not collect them, so every item reports none.
func (s *Server) GetThemeMedia(ctx context.Context, request api.GetThemeMediaRequestObject) (api.GetThemeMediaResponseObject, error) {
	return api.GetThemeMedia200JSONResponse{
		ThemeVideosResult:     dtos.Ptr(themeMedia(request.ItemId)),
		ThemeSongsResult:      dtos.Ptr(themeMedia(request.ItemId)),
		SoundtrackSongsResult: dtos.Ptr(themeMedia(request.ItemId)),
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
		StartIndex:       dtos.Ptr(int32(0)),
		TotalRecordCount: dtos.Ptr(int32(0)),
	}
}
