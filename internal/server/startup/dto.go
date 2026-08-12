package startup

import (
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/setup"
	"github.com/FreekingDean/gojellyfin/internal/users"
)

// Jellyfin keeps remote access in the network configuration, which the spec
// documents as an untyped named configuration.
const networkConfigurationKey = "network"

type networkConfiguration struct {
	EnableRemoteAccess bool `json:"EnableRemoteAccess"`
	EnableUPnP         bool `json:"EnableUPnP"`
}

func startupConfiguration(culture setup.Culture) api.StartupConfigurationDto {
	return api.StartupConfigurationDto{
		UICulture:                 apiutil.Ptr(culture.UICulture),
		MetadataCountryCode:       apiutil.Ptr(culture.MetadataCountryCode),
		PreferredMetadataLanguage: apiutil.Ptr(culture.PreferredMetadataLanguage),
	}
}

func startupUser(user *users.User) api.StartupUserDto {
	if user == nil {
		return api.StartupUserDto{}
	}

	return api.StartupUserDto{Name: apiutil.Ptr(user.Name)}
}
