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

// The vendored spec types DeviceProfile as a free-form object, so it is read
// back out of json rather than bound to a generated type.
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

	video := make([]items.Profile, 0, len(parsed.DirectPlayProfiles))
	for _, entry := range parsed.DirectPlayProfiles {
		if strings.EqualFold(entry.Type, string(api.DlnaProfileTypeVideo)) {
			video = append(video, entry.Profile)
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

	return items.Capabilities{Profiles: video, Conditions: conditions}
}

func streamURL(itemID uuid.UUID, plan items.Plan, token, session string, startTicks int64) string {
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
	// jellyfin-web retries a playback error until the url refuses both stream
	// copies. Nothing here reads them; they are what ends the retry.
	query.Set("allowVideoStreamCopy", "false")
	query.Set("allowAudioStreamCopy", "false")

	return fmt.Sprintf("/Videos/%s/stream.%s?%s", itemID, plan.Container, query.Encode())
}
