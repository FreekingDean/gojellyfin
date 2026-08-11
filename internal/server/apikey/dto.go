package apikey

import (
	"time"

	"github.com/FreekingDean/gojellyfin/internal/apikeys"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

func AuthenticationInfo(key *apikeys.ApiKey) api.AuthenticationInfo {
	info := api.AuthenticationInfo{
		AccessToken: apiutil.Ptr(key.AccessToken),
		AppName:     apiutil.Ptr(key.AppName),
		DateCreated: apiutil.Ptr(key.CreatedAt),
		IsActive:    apiutil.Ptr(true),
	}

	if !key.RevokedAt.IsZero() && key.RevokedAt.Before(time.Now()) {
		info.IsActive = apiutil.Ptr(false)
		info.DateRevoked = apiutil.Ptr(key.RevokedAt)
	}

	return info
}
