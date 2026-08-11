package plugins

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
)

// Plugins are not supported, so nothing is ever installed: reads come back
// empty and anything that would mutate an installation is refused.
type Server struct{}

func New() *Server {
	return &Server{}
}

func (s *Server) GetPlugins(ctx context.Context, request api.GetPluginsRequestObject) (api.GetPluginsResponseObject, error) {
	return api.GetPlugins200JSONResponse([]api.PluginInfo{}), nil
}

func (s *Server) GetPluginConfiguration(ctx context.Context, request api.GetPluginConfigurationRequestObject) (api.GetPluginConfigurationResponseObject, error) {
	return api.GetPluginConfiguration404JSONResponse{}, nil
}

func (s *Server) GetPluginManifest(ctx context.Context, request api.GetPluginManifestRequestObject) (api.GetPluginManifestResponseObject, error) {
	return api.GetPluginManifest404JSONResponse{}, nil
}

func (s *Server) GetPluginImage(ctx context.Context, request api.GetPluginImageRequestObject) (api.GetPluginImageResponseObject, error) {
	return api.GetPluginImage404JSONResponse{}, nil
}

func (s *Server) UpdatePluginConfiguration(ctx context.Context, request api.UpdatePluginConfigurationRequestObject) (api.UpdatePluginConfigurationResponseObject, error) {
	return api.UpdatePluginConfiguration404JSONResponse{}, nil
}

func (s *Server) UninstallPlugin(ctx context.Context, request api.UninstallPluginRequestObject) (api.UninstallPluginResponseObject, error) {
	return api.UninstallPlugin404JSONResponse{}, nil
}

func (s *Server) UninstallPluginByVersion(ctx context.Context, request api.UninstallPluginByVersionRequestObject) (api.UninstallPluginByVersionResponseObject, error) {
	return api.UninstallPluginByVersion404JSONResponse{}, nil
}

func (s *Server) EnablePlugin(ctx context.Context, request api.EnablePluginRequestObject) (api.EnablePluginResponseObject, error) {
	return api.EnablePlugin404JSONResponse{}, nil
}

func (s *Server) DisablePlugin(ctx context.Context, request api.DisablePluginRequestObject) (api.DisablePluginResponseObject, error) {
	return api.DisablePlugin404JSONResponse{}, nil
}
