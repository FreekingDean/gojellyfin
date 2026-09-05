package dashboard

import (
	"bytes"
	"context"
	_ "embed"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

//go:embed configpage.html
var configPage []byte

type Server struct{}

func New() *Server {
	return &Server{}
}

var pages = map[string]string{
	"media_downloaders": "Media Downloaders",
}

func (s *Server) GetConfigurationPages(
	_ context.Context,
	_ api.GetConfigurationPagesRequestObject,
) (api.GetConfigurationPagesResponseObject, error) {
	pageInfos := make([]api.ConfigurationPageInfo, 0, len(pages))
	for name, displayName := range pages {
		pluginID := uuid.NewMD5(uuid.NameSpaceOID, []byte(name))
		pageInfos = append(pageInfos, api.ConfigurationPageInfo{
			MenuIcon:    apiutil.Ptr("person"),
			DisplayName: apiutil.Ptr(displayName),
			Name:        apiutil.Ptr(name),
			PluginId:    apiutil.Ptr(pluginID),
		})
	}

	return api.GetConfigurationPages200JSONResponse(pageInfos), nil
}

func (s *Server) GetDashboardConfigurationPage(
	_ context.Context,
	_ api.GetDashboardConfigurationPageRequestObject,
) (api.GetDashboardConfigurationPageResponseObject, error) {
	return api.GetDashboardConfigurationPage200TexthtmlResponse{
		Body:          bytes.NewBuffer(configPage),
		ContentLength: int64(len(configPage)),
	}, nil
}
