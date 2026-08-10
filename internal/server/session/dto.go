package session

import (
	"github.com/FreekingDean/gojellyfin/internal/config"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/sessions"
)

func SessionDto(session *sessions.Session) *api.SessionInfoDto {
	dto := &api.SessionInfoDto{
		Id:                    apiutil.Ptr(session.ID.String()),
		ServerId:              apiutil.Ptr(config.ServerID),
		LastActivityDate:      apiutil.Ptr(session.LastActivityAt),
		IsActive:              apiutil.Ptr(true),
		SupportsRemoteControl: apiutil.Ptr(false),
		PlayableMediaTypes:    &[]api.MediaType{},
		SupportedCommands:     &[]api.GeneralCommandType{},
		AdditionalUsers:       &[]api.SessionUserInfo{},
	}

	if device := session.Edges.Device; device != nil {
		playable := make([]api.MediaType, 0, len(device.PlayableMediaTypes))
		for _, mediaType := range device.PlayableMediaTypes {
			playable = append(playable, api.MediaType(mediaType))
		}
		commands := make([]api.GeneralCommandType, 0, len(device.SupportedCommands))
		for _, command := range device.SupportedCommands {
			commands = append(commands, api.GeneralCommandType(command))
		}

		dto.Client = apiutil.Ptr(device.AppName)
		dto.DeviceId = apiutil.Ptr(device.ClientID)
		dto.DeviceName = apiutil.Ptr(device.Name)
		dto.ApplicationVersion = apiutil.Ptr(device.AppVersion)
		dto.SupportsRemoteControl = apiutil.Ptr(device.SupportsMediaControl)
		dto.PlayableMediaTypes = &playable
		dto.SupportedCommands = &commands
	}

	if user := session.Edges.User; user != nil {
		dto.UserId = &user.ID
		dto.UserName = apiutil.Ptr(user.Name)
	}

	return dto
}
