package dashboard

import (
	"bytes"
	"context"
	_ "embed"
	"strings"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

//go:embed configpage.html
var configPage []byte

//go:embed requestspage.html
var requestsPage []byte

type Server struct{}

func New() *Server {
	return &Server{}
}

type page struct {
	name        string
	displayName string
	icon        string
	body        []byte
}

var pages = []page{
	{name: "media_downloaders", displayName: "Media Downloaders", icon: "download", body: configPage},
	{name: "requests", displayName: "Requests and Reports", icon: "favorite", body: requestsPage},
}

func (s *Server) GetConfigurationPages(
	_ context.Context,
	_ api.GetConfigurationPagesRequestObject,
) (api.GetConfigurationPagesResponseObject, error) {
	pageInfos := make([]api.ConfigurationPageInfo, 0, len(pages))
	for _, listed := range pages {
		pluginID := uuid.NewMD5(uuid.NameSpaceOID, []byte(listed.name))
		pageInfos = append(pageInfos, api.ConfigurationPageInfo{
			MenuIcon:    apiutil.Ptr(listed.icon),
			DisplayName: apiutil.Ptr(listed.displayName),
			Name:        apiutil.Ptr(listed.name),
			PluginId:    apiutil.Ptr(pluginID),
		})
	}

	return api.GetConfigurationPages200JSONResponse(pageInfos), nil
}

func (s *Server) GetDashboardConfigurationPage(
	_ context.Context,
	request api.GetDashboardConfigurationPageRequestObject,
) (api.GetDashboardConfigurationPageResponseObject, error) {
	body := pages[0].body
	for _, listed := range pages {
		if strings.EqualFold(listed.name, apiutil.Deref(request.Params.Name)) {
			body = listed.body
		}
	}

	return api.GetDashboardConfigurationPage200TexthtmlResponse{
		Body:          bytes.NewBuffer(body),
		ContentLength: int64(len(body)),
	}, nil
}
