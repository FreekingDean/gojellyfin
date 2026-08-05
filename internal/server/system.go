package server

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
)

func (s *Server) GetPublicSystemInfo(
	ctx context.Context,
	request api.GetPublicSystemInfoRequestObject,
) (api.GetPublicSystemInfoResponseObject, error) {
	return api.GetPublicSystemInfo200JSONResponse{
		Id:                     ptr(s.ID()),
		LocalAddress:           ptr(s.system.LocalAddress()),
		ServerName:             ptr(s.Name()),
		ProductName:            ptr(s.system.ProductName()),
		Version:                ptr(s.system.Version()),
		StartupWizardCompleted: ptr(true),
		OperatingSystem:        ptr(s.system.OperatingSystem()),
	}, nil
}

func (s *Server) GetSystemInfo(ctx context.Context, request api.GetSystemInfoRequestObject) (api.GetSystemInfoResponseObject, error) {
	return api.GetSystemInfo200JSONResponse{
		Id:                       ptr(s.ID()),
		LocalAddress:             ptr(s.system.LocalAddress()),
		ServerName:               ptr(s.Name()),
		ProductName:              ptr(s.system.ProductName()),
		Version:                  ptr(s.system.Version()),
		CastReceiverApplications: ptr([]api.CastReceiverApplication{}),
		StartupWizardCompleted:   ptr(true),
		HasPendingRestart:        ptr(false),
		IsShuttingDown:           ptr(false),
		SupportsLibraryMonitor:   ptr(true),
		WebSocketPortNumber:      ptr(int32(8082)),
		CompletedInstallations:   ptr([]api.InstallationInfo{}),

		//Deprecated
		OperatingSystem:            ptr(s.system.OperatingSystem()),
		OperatingSystemDisplayName: ptr(s.system.OperatingSystem()),
		CanSelfRestart:             ptr(true),
		CanLaunchWebBrowser:        ptr(false),
		WebPath:                    ptr("/gojelly/jellyfin-web"),
		ItemsByNamePath:            ptr("/gojelly/items"),
		CachePath:                  ptr("/gojelly/cache"),
		LogPath:                    ptr("/gojelly/logs"),
		InternalMetadataPath:       ptr("/gojelly/metadata"),
		TranscodingTempPath:        ptr("/gojelly/transcoding"),
		HasUpdateAvailable:         ptr(false),
		EncoderLocation:            ptr(""),
		SystemArchitecture:         ptr("amd64"),
	}, nil
}
