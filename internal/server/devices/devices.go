package devices

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/users"
)

type Server struct {
	auth  *auth.Service
	users *users.Service
}

func New(auth *auth.Service, users *users.Service) *Server {
	return &Server{auth: auth, users: users}
}

func (s *Server) GetDevices(ctx context.Context, request api.GetDevicesRequestObject) (api.GetDevicesResponseObject, error) {
	sessions, err := s.auth.Devices(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]api.DeviceInfoDto, 0, len(sessions))
	for _, session := range sessions {
		device := api.DeviceInfoDto{
			Id:               apiutil.Ptr(session.DeviceID),
			Name:             apiutil.Ptr(session.DeviceName),
			AppName:          apiutil.Ptr(session.Client),
			AppVersion:       apiutil.Ptr(session.AppVersion),
			DateLastActivity: apiutil.Ptr(session.LastActivityDate),
			LastUserId:       apiutil.Ptr(session.UserID),
		}
		if user, err := s.users.User(ctx, session.UserID); err == nil {
			device.LastUserName = apiutil.Ptr(user.Name)
		}
		items = append(items, device)
	}

	return api.GetDevices200JSONResponse{
		Items:            &items,
		StartIndex:       apiutil.Ptr(int32(0)),
		TotalRecordCount: apiutil.Ptr(int32(len(items))),
	}, nil
}
