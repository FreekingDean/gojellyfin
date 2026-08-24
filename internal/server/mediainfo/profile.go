package mediainfo

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
)

type codecProfile struct {
	Type       string
	Codec      string
	Conditions []struct {
		Condition string
		Property  string
		Value     string
	}
}

// The spec types DeviceProfile as a free-form object, so what the client
// declared is read back out of it rather than bound to a generated type. Only
// the video lines are read: what a device can decode inside an mp3 says nothing
// about what it can decode inside an mkv.
func capabilities(profile api.DeviceProfile) items.Capabilities {
	if len(profile) == 0 {
		return items.Capabilities{}
	}

	declared, err := json.Marshal(profile)
	if err != nil {
		return items.Capabilities{}
	}

	var parsed struct {
		DirectPlayProfiles []struct {
			items.Profile
			Type string
		}
		CodecProfiles []codecProfile
	}
	if err := json.Unmarshal(declared, &parsed); err != nil {
		return items.Capabilities{}
	}

	video := make([]items.Profile, 0, len(parsed.DirectPlayProfiles))
	for _, entry := range parsed.DirectPlayProfiles {
		if strings.EqualFold(entry.Type, string(api.DlnaProfileTypeVideo)) {
			video = append(video, entry.Profile)
		}
	}

	return items.Capabilities{Profiles: video, Codecs: codecConditions(parsed.CodecProfiles)}
}

// What the client needs to be true of a picture beyond naming its codec. Only
// the video lines are read: a condition on the sound inside a video is a
// separate question this does not answer yet.
func codecConditions(declared []codecProfile) []items.CodecCondition {
	conditions := make([]items.CodecCondition, 0, len(declared))
	for _, entry := range declared {
		if !strings.EqualFold(entry.Type, string(api.DlnaProfileTypeVideo)) || len(entry.Conditions) == 0 {
			continue
		}

		read := make([]items.Condition, 0, len(entry.Conditions))
		for _, condition := range entry.Conditions {
			read = append(read, items.Condition{
				Property: condition.Property,
				Verb:     condition.Condition,
				Value:    condition.Value,
			})
		}

		conditions = append(conditions, items.CodecCondition{Codec: entry.Codec, Conditions: read})
	}

	return conditions
}

// The one url the client is given. It states the container and the two codecs
// the response will carry, which is the same triple the stream handler reads to
// decide what to copy and what to convert, so neither end has to guess.
func streamURL(itemID uuid.UUID, plan items.Plan, token, session string, startTicks int64) string {
	query := url.Values{
		"mediaSourceId": {plan.Source.ID.String()},
		"container":     {plan.Container},
		"playSessionId": {session},
		"api_key":       {token},
	}
	if plan.VideoCodec != "" {
		query.Set("videoCodec", plan.VideoCodec)
	}
	if plan.AudioCodec != "" {
		query.Set("audioCodec", plan.AudioCodec)
	}
	if startTicks > 0 {
		query.Set("startTimeTicks", strconv.FormatInt(startTicks, 10))
	}
	refuseStreamCopy(query)

	return fmt.Sprintf("/Videos/%s/stream.%s?%s", itemID, plan.Container, query.Encode())
}

// jellyfin-web answers a playback error by asking for the stream all over
// again, and stops only once the url it was handed refuses both stream copies.
// Nothing downstream reads these two: they are what ends the retry.
func refuseStreamCopy(query url.Values) {
	query.Set("allowVideoStreamCopy", "false")
	query.Set("allowAudioStreamCopy", "false")
}
