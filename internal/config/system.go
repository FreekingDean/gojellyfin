package config

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
)

func (s *Server) GetPublicSystemInfo(
	ctx context.Context,
	request api.GetPublicSystemInfoRequestObject,
) (api.GetPublicSystemInfoResponseObject, error) {
	configuration, err := s.serverConfiguration(ctx)
	if err != nil {
		return nil, err
	}

	return api.GetPublicSystemInfo200JSONResponse{
		Id:                     ptr(ServerID),
		LocalAddress:           ptr(s.system.LocalAddress()),
		ServerName:             configuration.ServerName,
		ProductName:            ptr(s.system.ProductName()),
		Version:                ptr(s.system.Version()),
		StartupWizardCompleted: configuration.IsStartupWizardCompleted,
		OperatingSystem:        ptr(s.system.OperatingSystem()),
	}, nil
}

func (s *Server) GetSystemInfo(ctx context.Context, request api.GetSystemInfoRequestObject) (api.GetSystemInfoResponseObject, error) {
	configuration, err := s.serverConfiguration(ctx)
	if err != nil {
		return nil, err
	}

	return api.GetSystemInfo200JSONResponse{
		Id:                       ptr(ServerID),
		LocalAddress:             ptr(s.system.LocalAddress()),
		ServerName:               configuration.ServerName,
		ProductName:              ptr(s.system.ProductName()),
		Version:                  ptr(s.system.Version()),
		CastReceiverApplications: configuration.CastReceiverApplications,
		StartupWizardCompleted:   configuration.IsStartupWizardCompleted,
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
		CachePath:                  orElse(configuration.CachePath, "/gojelly/cache"),
		LogPath:                    ptr("/gojelly/logs"),
		InternalMetadataPath:       orElse(configuration.MetadataPath, "/gojelly/metadata"),
		TranscodingTempPath:        ptr("/gojelly/transcoding"),
		HasUpdateAvailable:         ptr(false),
		EncoderLocation:            ptr(""),
		SystemArchitecture:         ptr("amd64"),
	}, nil
}
