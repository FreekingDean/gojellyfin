package session

import (
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/sessions"
)

func capabilities(reported api.ClientCapabilitiesDto) sessions.Capabilities {
	playable := make([]string, 0, len(apiutil.Deref(reported.PlayableMediaTypes)))
	for _, mediaType := range apiutil.Deref(reported.PlayableMediaTypes) {
		playable = append(playable, string(mediaType))
	}
	commands := make([]string, 0, len(apiutil.Deref(reported.SupportedCommands)))
	for _, command := range apiutil.Deref(reported.SupportedCommands) {
		commands = append(commands, string(command))
	}

	return sessions.Capabilities{
		PlayableMediaTypes:           playable,
		SupportedCommands:            commands,
		SupportsMediaControl:         apiutil.Deref(reported.SupportsMediaControl),
		SupportsPersistentIdentifier: apiutil.Deref(reported.SupportsPersistentIdentifier),
		Profile:                      apiutil.Deref(reported.DeviceProfile),
	}
}
