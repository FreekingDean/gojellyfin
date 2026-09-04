package dashboard

import (
	"bytes"
	"context"
	"os"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/google/uuid"
)

type Server struct{}

func New() *Server {
	return &Server{}
}

var pages = map[string]string{
	"media_downloaders": "Media Downloaders",
}

func (s *Server) GetConfigurationPages(ctx context.Context, request api.GetConfigurationPagesRequestObject) (api.GetConfigurationPagesResponseObject, error) {
	pageInfos := make([]api.ConfigurationPageInfo, 0, len(pages))
	for name, displayName := range pages {
		pluginID := uuid.NewMD5(uuid.New(), []byte(name))
		pageInfos = append(pageInfos, api.ConfigurationPageInfo{
			MenuIcon:    apiutil.Ptr("person"),
			DisplayName: apiutil.Ptr(displayName),
			Name:        apiutil.Ptr(name),
			PluginId:    apiutil.Ptr(pluginID),
		})
	}
	return api.GetConfigurationPages200JSONResponse(pageInfos), nil
}

func (s *Server) GetDashboardConfigurationPage(ctx context.Context, request api.GetDashboardConfigurationPageRequestObject) (api.GetDashboardConfigurationPageResponseObject, error) {
	data, err := os.ReadFile("./configpage.html")
	if err != nil {
		return nil, err
	}

	resp := api.GetDashboardConfigurationPage200TexthtmlResponse{
		Body:          bytes.NewBuffer(data),
		ContentLength: int64(len(data)),
	}
	return resp, nil
}
