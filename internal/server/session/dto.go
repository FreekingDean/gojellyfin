package session

import (
	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/config"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/users"
)

func SessionDto(session *auth.Session, user *users.User) *api.SessionInfoDto {
	dto := &api.SessionInfoDto{
		Id:                    apiutil.Ptr(session.ID.String()),
		ServerId:              apiutil.Ptr(config.ServerID),
		Client:                apiutil.Ptr(session.Client),
		DeviceId:              apiutil.Ptr(session.DeviceID),
		DeviceName:            apiutil.Ptr(session.DeviceName),
		ApplicationVersion:    apiutil.Ptr(session.AppVersion),
		LastActivityDate:      apiutil.Ptr(session.LastActivityDate),
		IsActive:              apiutil.Ptr(true),
		SupportsRemoteControl: apiutil.Ptr(false),
		PlayableMediaTypes:    &[]api.MediaType{},
		SupportedCommands:     &[]api.GeneralCommandType{},
		AdditionalUsers:       &[]api.SessionUserInfo{},
	}

	if user != nil {
		dto.UserId = &user.ID
		dto.UserName = apiutil.Ptr(user.Name)
	}

	return dto
}
