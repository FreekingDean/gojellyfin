package quickconnect

import (
	"github.com/FreekingDean/gojellyfin/internal/quickconnect"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

func resultDto(request quickconnect.Request) api.QuickConnectResult {
	return api.QuickConnectResult{
		Authenticated: apiutil.Ptr(request.Authenticated()),
		Secret:        apiutil.Ptr(request.Secret),
		Code:          apiutil.Ptr(request.Code),
		DeviceId:      apiutil.Ptr(request.Device.ID),
		DeviceName:    apiutil.Ptr(request.Device.Name),
		AppName:       apiutil.Ptr(request.Device.AppName),
		AppVersion:    apiutil.Ptr(request.Device.AppVersion),
		DateAdded:     apiutil.Ptr(request.AddedAt),
	}
}
