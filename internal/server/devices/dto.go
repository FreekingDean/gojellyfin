package devices

import (
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/sessions"
)

func DeviceInfoDto(device *sessions.Device) api.DeviceInfoDto {
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

	return info
}

func DeviceOptionsDto(device *sessions.Device) api.DeviceOptionsDto {
	return api.DeviceOptionsDto{
		DeviceId:   apiutil.Ptr(device.ClientID),
		CustomName: apiutil.Ptr(device.CustomName),
	}
}
