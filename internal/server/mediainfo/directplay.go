package mediainfo

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	streammodal "github.com/FreekingDean/gojellyfin/internal/store/mediastream"
	"github.com/FreekingDean/gojellyfin/internal/transcode"
)

// The spec types DeviceProfile as a free-form object, so what the client
// declared is read back out of it rather than bound to a generated type.
type playProfile struct {
	Container  string
	AudioCodec string
	Type       string
}

func videoProfiles(profile api.DeviceProfile) []playProfile {
	if len(profile) == 0 {
		return nil
	}

	declared, err := json.Marshal(profile)
	if err != nil {
		return nil
	}

	var parsed struct{ DirectPlayProfiles []playProfile }
	if err := json.Unmarshal(declared, &parsed); err != nil {
		return nil
	}

	video := make([]playProfile, 0, len(parsed.DirectPlayProfiles))
	for _, entry := range parsed.DirectPlayProfiles {
		if strings.EqualFold(entry.Type, string(api.DlnaProfileTypeVideo)) {
			video = append(video, entry)
		}
	}

	return video
}

func directPlays(profiles []playProfile, source *items.MediaSource) bool {
	if len(profiles) == 0 {
		return true
	}

	container := strings.ToLower(source.Container)
	codec := strings.ToLower(firstAudioCodec(source))
	for _, profile := range profiles {
		if lists(profile.Container, container) && (codec == "" || lists(profile.AudioCodec, codec)) {
			return true
		}
	}

	return false
}

func lists(declared, value string) bool {
	if declared == "" {
		return true
	}

	for _, entry := range strings.Split(declared, ",") {
		if strings.EqualFold(strings.TrimSpace(entry), value) {
			return true
		}
	}

	return false
}

// The track the remux keeps: ffmpeg maps the first audio stream, so that is the
// one the client has to be able to decode.
func firstAudioCodec(source *items.MediaSource) string {
	codec := ""
	index := int32(0)
	for _, stream := range source.Edges.Streams {
		if stream.Kind != streammodal.KindAudio {
			continue
		}
		if codec == "" || stream.Index < index {
			codec, index = stream.Codec, stream.Index
		}
	}

	return codec
}

func transcodingURL(itemID uuid.UUID, source *items.MediaSource, token, session string) string {
	query := url.Values{
		"mediaSourceId": {source.ID.String()},
		"container":     {transcode.VideoContainer},
		"playSessionId": {session},
		"api_key":       {token},
	}

	return fmt.Sprintf("/Videos/%s/stream.%s?%s", itemID, transcode.VideoContainer, query.Encode())
}
