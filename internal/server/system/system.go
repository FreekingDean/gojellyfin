package system

import (
	"context"
	"fmt"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/config"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/server/configuration"
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
	configuration, err := configuration.ServerConfiguration(ctx, s.config)
	if err != nil {
		return nil, err
	}

	return api.GetPublicSystemInfo200JSONResponse{
		Id:                     apiutil.Ptr(config.ServerID),
		LocalAddress:           apiutil.Ptr(s.system.LocalAddress()),
		ServerName:             configuration.ServerName,
		ProductName:            apiutil.Ptr(s.system.ProductName()),
		Version:                apiutil.Ptr(s.system.Version()),
		StartupWizardCompleted: configuration.IsStartupWizardCompleted,
		OperatingSystem:        apiutil.Ptr(s.system.OperatingSystem()),
	}, nil
}

func (s *Server) GetSystemInfo(ctx context.Context, request api.GetSystemInfoRequestObject) (api.GetSystemInfoResponseObject, error) {
	configuration, err := configuration.ServerConfiguration(ctx, s.config)
	if err != nil {
		return nil, err
	}

	return api.GetSystemInfo200JSONResponse{
		Id:                       apiutil.Ptr(config.ServerID),
		LocalAddress:             apiutil.Ptr(s.system.LocalAddress()),
		ServerName:               configuration.ServerName,
		ProductName:              apiutil.Ptr(s.system.ProductName()),
		PackageName:              apiutil.Ptr(s.system.PackageName()),
		Version:                  apiutil.Ptr(s.system.Version()),
		CastReceiverApplications: configuration.CastReceiverApplications,
		StartupWizardCompleted:   configuration.IsStartupWizardCompleted,
		HasPendingRestart:        apiutil.Ptr(false),
		IsShuttingDown:           apiutil.Ptr(false),
		SupportsLibraryMonitor:   apiutil.Ptr(true),
		WebSocketPortNumber:      apiutil.Ptr(int32(8082)),
		CompletedInstallations:   apiutil.Ptr([]api.InstallationInfo{}),

		OperatingSystem:            apiutil.Ptr(s.system.OperatingSystem()),
		OperatingSystemDisplayName: apiutil.Ptr(s.system.OperatingSystem()),
		CanSelfRestart:             apiutil.Ptr(false),
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

func (s *Server) GetSystemStorage(ctx context.Context, request api.GetSystemStorageRequestObject) (api.GetSystemStorageResponseObject, error) {
	return api.GetSystemStorage200JSONResponse{
		CacheFolder:            storageDto("cache"),
		ImageCacheFolder:       storageDto("imagecache"),
		InternalMetadataFolder: storageDto("internalmetadata"),
		Libraries:              []api.LibraryStorageDto{},
		LogFolder:              storageDto("logs"),
		TranscodingTempFolder:  storageDto("transcoding"),
		WebFolder:              storageDto("web"),
		ProgramDataFolder:      storageDto("programdata"),
	}, nil
}

func storageDto(name string) api.FolderStorageDto {
	return api.FolderStorageDto{
		DeviceId:    apiutil.Ptr(fmt.Sprintf("UUID-%s", name)),
		FreeSpace:   apiutil.Ptr(int64(1000000000)),
		Path:        fmt.Sprintf("/gojelly/%s", name),
		StorageType: apiutil.Ptr("Fixed"),
		UsedSpace:   apiutil.Ptr(int64(100000000)),
	}
}

func (s *Server) GetPingSystem(ctx context.Context, request api.GetPingSystemRequestObject) (api.GetPingSystemResponseObject, error) {
	return api.GetPingSystem200JSONResponse(s.system.ProductName()), nil
}

func (s *Server) PostPingSystem(ctx context.Context, request api.PostPingSystemRequestObject) (api.PostPingSystemResponseObject, error) {
	return api.PostPingSystem200JSONResponse(s.system.ProductName()), nil
}

func (s *Server) GetEndpointInfo(ctx context.Context, request api.GetEndpointInfoRequestObject) (api.GetEndpointInfoResponseObject, error) {
	return api.GetEndpointInfo200JSONResponse(endpointInfo(auth.AuthorizationFrom(ctx).RemoteAddr)), nil
}
