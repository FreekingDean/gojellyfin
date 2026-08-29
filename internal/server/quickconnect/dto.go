package quickconnect

import (
	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/quickconnect"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

func resultDto(request *quickconnect.Request) api.QuickConnectResult {
	return api.QuickConnectResult{
		Authenticated: apiutil.Ptr(request.AuthorizedByID != uuid.Nil),
		Secret:        apiutil.Ptr(request.Secret),
		Code:          apiutil.Ptr(request.Code),
		DeviceId:      apiutil.Ptr(request.DeviceID),
		DeviceName:    apiutil.Ptr(request.DeviceName),
		AppName:       apiutil.Ptr(request.AppName),
		AppVersion:    apiutil.Ptr(request.AppVersion),
		DateAdded:     apiutil.Ptr(request.CreatedAt),
	}
}
