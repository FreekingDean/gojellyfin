package server

import (
	"bytes"
	"context"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
)

func (s *Server) GetLogEntries(ctx context.Context, request api.GetLogEntriesRequestObject) (api.GetLogEntriesResponseObject, error) {
	return api.GetLogEntries200JSONResponse{}, nil
}

func (s *Server) AddItemToPlaylist(ctx context.Context, request api.AddItemToPlaylistRequestObject) (api.AddItemToPlaylistResponseObject, error) {
	return api.AddItemToPlaylist204Response{}, nil
}

func (s *Server) GetQuickConnectEnabled(ctx context.Context, request api.GetQuickConnectEnabledRequestObject) (api.GetQuickConnectEnabledResponseObject, error) {
	return api.GetQuickConnectEnabled200JSONResponse(true), nil
}

func (s *Server) GetDisplayPreferences(ctx context.Context, request api.GetDisplayPreferencesRequestObject) (api.GetDisplayPreferencesResponseObject, error) {
	return api.GetDisplayPreferences200JSONResponse{}, nil
}

func (s *Server) GetUserViews(ctx context.Context, request api.GetUserViewsRequestObject) (api.GetUserViewsResponseObject, error) {
	items := []api.BaseItemDto{{
		Id:   uid("4cb0ebf115cb44068837635374d3a6ea"),
		Name: ptr("Default"),
	}}
	return api.GetUserViews200JSONResponse{
		Items:            &items,
		StartIndex:       ptr(int32(0)),
		TotalRecordCount: ptr(int32(len(items))),
	}, nil
}

func (s *Server) GetBitrateTestBytes(ctx context.Context, request api.GetBitrateTestBytesRequestObject) (api.GetBitrateTestBytesResponseObject, error) {
	buf := bytes.NewBufferString("This is a test endpoint info response")
	return api.GetBitrateTestBytes200ApplicationoctetStreamResponse{
		Body:          buf,
		ContentLength: int64(buf.Len()),
	}, nil
}

func (s *Server) GetEndpointInfo(ctx context.Context, request api.GetEndpointInfoRequestObject) (api.GetEndpointInfoResponseObject, error) {
	return api.GetEndpointInfo200JSONResponse{}, nil
}

func (s *Server) SyncPlayGetGroups(ctx context.Context, request api.SyncPlayGetGroupsRequestObject) (api.SyncPlayGetGroupsResponseObject, error) {
	return api.SyncPlayGetGroups200JSONResponse{}, nil
}
