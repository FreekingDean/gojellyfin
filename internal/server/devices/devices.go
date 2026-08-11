package devices

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/sessions"
)

type Server struct {
	sessions *sessions.Service
}

func New(sessions *sessions.Service) *Server {
	return &Server{sessions: sessions}
}

func (s *Server) GetDevices(ctx context.Context, request api.GetDevicesRequestObject) (api.GetDevicesResponseObject, error) {
	devices, err := s.sessions.Devices(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]api.DeviceInfoDto, 0, len(devices))
	for _, device := range devices {
		info := api.DeviceInfoDto{
			Id:               apiutil.Ptr(device.ClientID),
			Name:             apiutil.Ptr(device.Name),
			CustomName:       apiutil.Ptr(device.CustomName),
			AppName:          apiutil.Ptr(device.AppName),
			AppVersion:       apiutil.Ptr(device.AppVersion),
			DateLastActivity: apiutil.Ptr(device.LastActivityAt),
			IconUrl:          apiutil.Ptr(device.IconURL),
		}
		if user := sessions.LastUser(device); user != nil {
			info.LastUserId = &user.ID
			info.LastUserName = apiutil.Ptr(user.Name)
		}
		items = append(items, info)
	}

	return api.GetDevices200JSONResponse{
		Items:            &items,
		StartIndex:       apiutil.Ptr(int32(0)),
		TotalRecordCount: apiutil.Ptr(int32(len(items))),
	}, nil
}
