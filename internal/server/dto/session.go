package dto

import (
	"time"

	"github.com/FreekingDean/gojellyfin/internal/config"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/sessions"
	"github.com/FreekingDean/gojellyfin/internal/transcode"
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

	dto.Capabilities = &api.ClientCapabilitiesDto{
		PlayableMediaTypes:           dto.PlayableMediaTypes,
		SupportedCommands:            dto.SupportedCommands,
		SupportsMediaControl:         dto.SupportsMediaControl,
		SupportsPersistentIdentifier: apiutil.Ptr(false),
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
		if source, plan := sessions.Planned(session); source != nil {
			planned(dto, &item, source, plan)
		}
	}

	if user := session.User; user != nil {
		dto.UserId = &user.ID
		dto.UserName = apiutil.Ptr(user.Name)
	}

	additional := make([]api.SessionUserInfo, 0, len(session.AdditionalUsers))
	for _, user := range session.AdditionalUsers {
		additional = append(additional, api.SessionUserInfo{UserId: apiutil.Ptr(user.ID), UserName: apiutil.Ptr(user.Name)})
	}
	dto.AdditionalUsers = &additional

	if sessions.NowViewing(session, time.Now()) {
		dto.NowViewingItem = apiutil.Ptr(ItemDto(session.NowViewing, items.Held{}, 0, nil))
	}

	return dto
}

func planned(dto *api.SessionInfoDto, item *api.BaseItemDto, source *items.MediaSource, plan *items.Playback) {
	listed := MediaSourceDto(source)
	listed.Id = apiutil.Ptr(item.Id.String())
	item.MediaSources = &[]api.MediaSourceInfo{listed}
	item.MediaStreams = listed.MediaStreams
	dto.PlayState.MediaSourceId = listed.Id

	if plan.Direct {
		dto.PlayState.PlayMethod = apiutil.Ptr(api.DirectPlay)

		return
	}

	reasons := make([]api.TranscodeReason, 0, len(plan.Reasons))
	for _, reason := range plan.Reasons {
		reasons = append(reasons, api.TranscodeReason(reason))
	}
	dto.PlayState.PlayMethod = apiutil.Ptr(api.Transcode)
	dto.TranscodingInfo = &api.TranscodingInfo{
		IsVideoDirect:    apiutil.Ptr(plan.VideoDirect),
		IsAudioDirect:    apiutil.Ptr(plan.AudioDirect),
		Container:        apiutil.ZeroOrNilPtr(plan.Container),
		VideoCodec:       apiutil.ZeroOrNilPtr(plan.VideoCodec),
		AudioCodec:       apiutil.ZeroOrNilPtr(plan.AudioCodec),
		TranscodeReasons: &reasons,
	}
	if !plan.VideoDirect {
		if height := items.Height(source); height > 0 {
			dto.TranscodingInfo.Height = apiutil.Ptr(min(height, transcode.MaxHeight))
		}
	}
}

func seriesOf(item *items.Item) *items.Item {
	if item.Kind != items.Kind(api.BaseItemKindEpisode) || item.Parent == nil {
		return nil
	}

	return item.Parent.Parent
}
