package apikey

import (
	"github.com/FreekingDean/gojellyfin/internal/apikeys"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

func AuthenticationInfo(key *apikeys.ApiKey) api.AuthenticationInfo {
	return api.AuthenticationInfo{
		AccessToken: apiutil.Ptr(key.AccessToken),
		AppName:     apiutil.Ptr(key.AppName),
		DateCreated: apiutil.Ptr(key.CreatedAt),
	}
}
