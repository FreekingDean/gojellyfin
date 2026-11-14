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
	"github.com/FreekingDean/gojellyfin/internal/transcode"
)

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
		TranscodingProfiles []struct {
			Type      string
			Container string
			Protocol  string
		}
		CodecProfiles []struct {
			Type       string
			Codec      string
			Conditions []struct {
				Condition string
				Property  string
				Value     string
			}
		}
	}
	if err := json.Unmarshal(declared, &parsed); err != nil {
		return items.Capabilities{}
	}

	var video, audio []items.Profile
	for _, entry := range parsed.DirectPlayProfiles {
		switch {
		case strings.EqualFold(entry.Type, string(api.DlnaProfileTypeVideo)):
			video = append(video, entry.Profile)
		case strings.EqualFold(entry.Type, string(api.DlnaProfileTypeAudio)):
			audio = append(audio, entry.Profile)
		}
	}

	var encodes []string
	for _, entry := range parsed.TranscodingProfiles {
		if strings.EqualFold(entry.Type, string(api.DlnaProfileTypeAudio)) && !strings.EqualFold(entry.Protocol, "hls") {
			encodes = append(encodes, strings.ToLower(strings.TrimSpace(entry.Container)))
		}
	}

	conditions := make([]items.Condition, 0, len(parsed.CodecProfiles))
	for _, entry := range parsed.CodecProfiles {
		if !strings.EqualFold(entry.Type, string(api.DlnaProfileTypeVideo)) {
			continue
		}
		for _, condition := range entry.Conditions {
			conditions = append(conditions, items.Condition{
				Codec:    entry.Codec,
				Property: condition.Property,
				Verb:     condition.Condition,
				Value:    condition.Value,
			})
		}
	}

	return items.Capabilities{Profiles: video, Conditions: conditions, Audio: audio, AudioEncodes: encodes}
}

func streamURL(root string, itemID uuid.UUID, plan items.Plan, token, session string, startTicks int64) string {
	query := url.Values{
		"mediaSourceId": {plan.Source.ID.String()},
		"container":     {plan.Container},
		"playSessionId": {session},
		"api_key":       {token},
	}
	if plan.AudioCodec != "" {
		query.Set("audioCodec", plan.AudioCodec)
	}
	if startTicks > 0 {
		query.Set("startTimeTicks", strconv.FormatInt(startTicks, 10))
	}
	if plan.VideoCodec != "" {
		query.Set("videoCodec", plan.VideoCodec)
		query.Set("maxHeight", strconv.Itoa(transcode.MaxHeight))
	}
	if plan.VideoCodec == "" && plan.Container == items.Container(plan.Source) && plan.AudioCodec == items.AudioCodec(plan.Source) {
		query.Set("copyTimestamps", "true")
	}
	query.Set("allowVideoStreamCopy", "false")
	query.Set("allowAudioStreamCopy", "false")

	return fmt.Sprintf("/%s/%s/stream.%s?%s", root, itemID, plan.Container, query.Encode())
}
