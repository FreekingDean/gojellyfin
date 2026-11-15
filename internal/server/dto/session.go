package dto

import (
	"time"

	"github.com/FreekingDean/gojellyfin/internal/config"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/sessions"
)

func SessionDto(session *sessions.Session) *api.SessionInfoDto {
	dto := &api.SessionInfoDto{
		Id:                    apiutil.Ptr(session.ID.String()),
		ServerId:              apiutil.Ptr(config.ServerID),
		LastActivityDate:      session.LastActivityAt,
		LastPlaybackCheckIn:   &time.Time{},
		IsActive:              apiutil.Ptr(true),
		SupportsRemoteControl: apiutil.Ptr(false),
		SupportsMediaControl:  apiutil.Ptr(false),
		HasCustomDeviceName:   apiutil.Ptr(false),
		PlayableMediaTypes:    &[]api.MediaType{},
		SupportedCommands:     &[]api.GeneralCommandType{},
		AdditionalUsers:       &[]api.SessionUserInfo{},
	}

	if device := session.Device; device != nil {
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
		dto.SupportsMediaControl = apiutil.Ptr(device.SupportsMediaControl)
		dto.HasCustomDeviceName = apiutil.Ptr(device.CustomName != "")
		dto.PlayableMediaTypes = &playable
		dto.SupportedCommands = &commands
	}

	if session.RemoteEndpoint != "" {
		dto.RemoteEndPoint = apiutil.Ptr(session.RemoteEndpoint)
	}

	dto.PlayState = &api.PlayerStateInfo{
		CanSeek:       apiutil.Ptr(false),
		IsPaused:      apiutil.Ptr(false),
		IsMuted:       apiutil.Ptr(false),
		RepeatMode:    apiutil.Ptr(api.RepeatModeRepeatNone),
		PlaybackOrder: apiutil.Ptr(api.PlaybackOrderDefault),
	}
	if sessions.NowPlaying(session, time.Now()) {
		item := ItemDto(session.NowPlaying, items.Held{}, 0, nil)
		if series := seriesOf(session.NowPlaying); series != nil {
			item.SeriesName = apiutil.Ptr(series.Name)
			item.SeriesId = &series.ID
		}
		dto.NowPlayingItem = &item
		dto.PlayState.CanSeek = apiutil.Ptr(true)
		dto.PlayState.IsPaused = apiutil.Ptr(session.IsPaused)
		dto.PlayState.PositionTicks = apiutil.Ptr(session.PositionTicks)
		dto.LastPlaybackCheckIn = session.PlayingAt
		if session.PlayMethod != "" {
			dto.PlayState.PlayMethod = apiutil.Ptr(api.PlayMethod(session.PlayMethod))
		}
	}

	if user := session.User; user != nil {
		dto.UserId = &user.ID
		dto.UserName = apiutil.Ptr(user.Name)
	}

	return dto
}

func seriesOf(item *items.Item) *items.Item {
	if item.Kind != items.Kind(api.BaseItemKindEpisode) || item.Parent == nil {
		return nil
	}

	return item.Parent.Parent
}
