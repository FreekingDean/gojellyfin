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

type declaredProfile struct {
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

func capabilities(profile api.DeviceProfile) items.Capabilities {
	if len(profile) == 0 {
		return items.Capabilities{}
	}

	declared, err := json.Marshal(profile)
	if err != nil {
		return items.Capabilities{}
	}

	var parsed declaredProfile
	if err := json.Unmarshal(declared, &parsed); err != nil {
		return items.Capabilities{}
	}

	video, audio := parsed.directPlay()

	return items.Capabilities{Profiles: video, Conditions: parsed.conditions(), Audio: audio, AudioEncodes: parsed.audioEncodes()}
}

func (p declaredProfile) directPlay() (video, audio []items.Profile) {
	for _, entry := range p.DirectPlayProfiles {
		switch {
		case strings.EqualFold(entry.Type, string(api.DlnaProfileTypeVideo)):
			video = append(video, entry.Profile)
		case strings.EqualFold(entry.Type, string(api.DlnaProfileTypeAudio)):
			audio = append(audio, entry.Profile)
		}
	}

	return video, audio
}

func (p declaredProfile) audioEncodes() []string {
	var encodes []string
	for _, entry := range p.TranscodingProfiles {
		if strings.EqualFold(entry.Type, string(api.DlnaProfileTypeAudio)) && !strings.EqualFold(entry.Protocol, "hls") {
			encodes = append(encodes, strings.ToLower(strings.TrimSpace(entry.Container)))
		}
	}

	return encodes
}

func (p declaredProfile) conditions() []items.Condition {
	conditions := make([]items.Condition, 0, len(p.CodecProfiles))
	for _, entry := range p.CodecProfiles {
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

	return conditions
}

func conversion(plan items.Plan) url.Values {
	query := url.Values{}
	if plan.AudioCodec != "" {
		query.Set("audioCodec", plan.AudioCodec)
	}
	if plan.VideoCodec != "" {
		query.Set("videoCodec", plan.VideoCodec)
		query.Set("maxHeight", strconv.Itoa(transcode.MaxHeight))
	}

	return query
}

func streamURL(root string, itemID uuid.UUID, plan items.Plan, token, session string, startTicks int64) string {
	query := instruction(plan, token, session, startTicks)
	query.Set("container", plan.Container)
	if plan.VideoCodec == "" && plan.Container == items.Container(plan.Source) && plan.AudioCodec == items.AudioCodec(plan.Source) {
		query.Set("copyTimestamps", "true")
	}

	return fmt.Sprintf("/%s/%s/stream.%s?%s", root, itemID, plan.Container, query.Encode())
}

func playlistURL(itemID uuid.UUID, plan items.Plan, token, session string, startTicks, from int64) string {
	query := instruction(plan, token, session, startTicks)
	if from > 0 {
		query.Set("fromTicks", strconv.FormatInt(from, 10))
	}

	return fmt.Sprintf("/Videos/%s/master.m3u8?%s", itemID, query.Encode())
}

func instruction(plan items.Plan, token, session string, startTicks int64) url.Values {
	query := conversion(plan)
	query.Set("mediaSourceId", plan.Source.ID.String())
	query.Set("playSessionId", session)
	query.Set("api_key", token)
	if startTicks > 0 {
		query.Set("startTimeTicks", strconv.FormatInt(startTicks, 10))
	}
	query.Set("allowVideoStreamCopy", "false")
	query.Set("allowAudioStreamCopy", "false")

	return query
}
