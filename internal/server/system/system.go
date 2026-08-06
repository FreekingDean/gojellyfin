package system

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/config"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/dtos"
	systemsvc "github.com/FreekingDean/gojellyfin/internal/system"
)

type Server struct {
	config *config.Service
	system systemsvc.Service
}

func New(config *config.Service, system systemsvc.Service) *Server {
	return &Server{config: config, system: system}
}

func (s *Server) GetPublicSystemInfo(
	ctx context.Context,
	request api.GetPublicSystemInfoRequestObject,
) (api.GetPublicSystemInfoResponseObject, error) {
	configuration, err := dtos.ServerConfiguration(ctx, s.config)
	if err != nil {
		return nil, err
	}

	return api.GetPublicSystemInfo200JSONResponse{
		Id:                     dtos.Ptr(config.ServerID),
		LocalAddress:           dtos.Ptr(s.system.LocalAddress()),
		ServerName:             configuration.ServerName,
		ProductName:            dtos.Ptr(s.system.ProductName()),
		Version:                dtos.Ptr(s.system.Version()),
		StartupWizardCompleted: configuration.IsStartupWizardCompleted,
		OperatingSystem:        dtos.Ptr(s.system.OperatingSystem()),
	}, nil
}

func (s *Server) GetSystemInfo(ctx context.Context, request api.GetSystemInfoRequestObject) (api.GetSystemInfoResponseObject, error) {
	configuration, err := dtos.ServerConfiguration(ctx, s.config)
	if err != nil {
		return nil, err
	}

	return api.GetSystemInfo200JSONResponse{
		Id:                       dtos.Ptr(config.ServerID),
		LocalAddress:             dtos.Ptr(s.system.LocalAddress()),
		ServerName:               configuration.ServerName,
		ProductName:              dtos.Ptr(s.system.ProductName()),
		Version:                  dtos.Ptr(s.system.Version()),
		CastReceiverApplications: configuration.CastReceiverApplications,
		StartupWizardCompleted:   configuration.IsStartupWizardCompleted,
		HasPendingRestart:        dtos.Ptr(false),
		IsShuttingDown:           dtos.Ptr(false),
		SupportsLibraryMonitor:   dtos.Ptr(true),
		WebSocketPortNumber:      dtos.Ptr(int32(8082)),
		CompletedInstallations:   dtos.Ptr([]api.InstallationInfo{}),

		//Deprecated
		OperatingSystem:            dtos.Ptr(s.system.OperatingSystem()),
		OperatingSystemDisplayName: dtos.Ptr(s.system.OperatingSystem()),
		CanSelfRestart:             dtos.Ptr(true),
		CanLaunchWebBrowser:        dtos.Ptr(false),
		WebPath:                    dtos.Ptr("/gojelly/jellyfin-web"),
		ItemsByNamePath:            dtos.Ptr("/gojelly/items"),
		CachePath:                  dtos.OrElse(configuration.CachePath, "/gojelly/cache"),
		LogPath:                    dtos.Ptr("/gojelly/logs"),
		InternalMetadataPath:       dtos.OrElse(configuration.MetadataPath, "/gojelly/metadata"),
		TranscodingTempPath:        dtos.Ptr("/gojelly/transcoding"),
		HasUpdateAvailable:         dtos.Ptr(false),
		EncoderLocation:            dtos.Ptr(""),
		SystemArchitecture:         dtos.Ptr("amd64"),
	}, nil
}
