package system

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/config"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/server/configuration"
	"github.com/FreekingDean/gojellyfin/internal/server/startup"
	systemsvc "github.com/FreekingDean/gojellyfin/internal/system"
	"github.com/FreekingDean/gojellyfin/internal/users"
)

type Server struct {
	config *config.Service
	system systemsvc.Service
	users  *users.Service
}

func New(config *config.Service, system systemsvc.Service, users *users.Service) *Server {
	return &Server{config: config, system: system, users: users}
}

func (s *Server) GetPublicSystemInfo(
	ctx context.Context,
	request api.GetPublicSystemInfoRequestObject,
) (api.GetPublicSystemInfoResponseObject, error) {
	configuration, err := configuration.ServerConfiguration(ctx, s.config)
	if err != nil {
		return nil, err
	}

	completed, err := startup.Completed(ctx, s.config, s.users)
	if err != nil {
		return nil, err
	}

	return api.GetPublicSystemInfo200JSONResponse{
		Id:                     apiutil.Ptr(config.ServerID),
		LocalAddress:           apiutil.Ptr(s.system.LocalAddress()),
		ServerName:             configuration.ServerName,
		ProductName:            apiutil.Ptr(s.system.ProductName()),
		Version:                apiutil.Ptr(s.system.Version()),
		StartupWizardCompleted: apiutil.Ptr(completed),
		OperatingSystem:        apiutil.Ptr(s.system.OperatingSystem()),
	}, nil
}

func (s *Server) GetSystemInfo(ctx context.Context, request api.GetSystemInfoRequestObject) (api.GetSystemInfoResponseObject, error) {
	configuration, err := configuration.ServerConfiguration(ctx, s.config)
	if err != nil {
		return nil, err
	}

	completed, err := startup.Completed(ctx, s.config, s.users)
	if err != nil {
		return nil, err
	}

	return api.GetSystemInfo200JSONResponse{
		Id:                       apiutil.Ptr(config.ServerID),
		LocalAddress:             apiutil.Ptr(s.system.LocalAddress()),
		ServerName:               configuration.ServerName,
		ProductName:              apiutil.Ptr(s.system.ProductName()),
		Version:                  apiutil.Ptr(s.system.Version()),
		CastReceiverApplications: configuration.CastReceiverApplications,
		StartupWizardCompleted:   apiutil.Ptr(completed),
		HasPendingRestart:        apiutil.Ptr(false),
		IsShuttingDown:           apiutil.Ptr(false),
		SupportsLibraryMonitor:   apiutil.Ptr(true),
		WebSocketPortNumber:      apiutil.Ptr(int32(8082)),
		CompletedInstallations:   apiutil.Ptr([]api.InstallationInfo{}),

		//Deprecated
		OperatingSystem:            apiutil.Ptr(s.system.OperatingSystem()),
		OperatingSystemDisplayName: apiutil.Ptr(s.system.OperatingSystem()),
		CanSelfRestart:             apiutil.Ptr(true),
		CanLaunchWebBrowser:        apiutil.Ptr(false),
		WebPath:                    apiutil.Ptr("/gojelly/jellyfin-web"),
		ItemsByNamePath:            apiutil.Ptr("/gojelly/items"),
		CachePath:                  apiutil.OrElse(configuration.CachePath, "/gojelly/cache"),
		LogPath:                    apiutil.Ptr("/gojelly/logs"),
		InternalMetadataPath:       apiutil.OrElse(configuration.MetadataPath, "/gojelly/metadata"),
		TranscodingTempPath:        apiutil.Ptr("/gojelly/transcoding"),
		HasUpdateAvailable:         apiutil.Ptr(false),
		EncoderLocation:            apiutil.Ptr(""),
		SystemArchitecture:         apiutil.Ptr("amd64"),
	}, nil
}
