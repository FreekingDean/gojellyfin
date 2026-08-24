package mediainfo

import (
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"strconv"
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

// What the one url will answer with, which is also what the client is told to
// expect: the source untouched, or the container and audio a remux lands in.
type delivery struct {
	container  string
	audioCodec string
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

	container := sourceContainer(source)
	codec := strings.ToLower(firstAudioCodec(source))
	for _, profile := range profiles {
		if lists(profile.Container, container) && (codec == "" || lists(profile.AudioCodec, codec)) {
			return true
		}
	}

	return false
}

func plan(profiles []playProfile, source *items.MediaSource) delivery {
	if directPlays(profiles, source) {
		return delivery{container: sourceContainer(source), audioCodec: firstAudioCodec(source)}
	}

	container := remuxContainer(profiles)

	return delivery{container: container, audioCodec: transcode.AudioCodec(container)}
}

// The container both ends agree on: one the client declared it can decode, and
// one ffmpeg can write down a pipe that is never seeked. A client declaring
// none leaves the default, which is what a browser takes.
func remuxContainer(profiles []playProfile) string {
	declared := make([]string, 0, len(profiles))
	for _, profile := range profiles {
		for _, container := range strings.Split(profile.Container, ",") {
			container = strings.TrimSpace(container)
			if lists(profile.AudioCodec, transcode.AudioCodec(container)) {
				declared = append(declared, container)
			}
		}
	}

	return transcode.ChooseVideo(append(declared, transcode.VideoContainer))
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

// The track a remux keeps: ffmpeg maps the first audio stream, so that is the
// one the client has to be able to decode.
func firstAudioCodec(source *items.MediaSource) string {
	codec := ""
	index := int32(0)
	for _, stream := range source.Edges.Streams {
		if stream.Kind != streammodal.KindAudio {
			continue
		}
		if codec == "" || stream.Index < index {
			codec, index = strings.ToLower(stream.Codec), stream.Index
		}
	}

	return codec
}

func sourceContainer(source *items.MediaSource) string {
	if source.Container != "" {
		return strings.ToLower(source.Container)
	}

	return strings.TrimPrefix(strings.ToLower(filepath.Ext(source.Path)), ".")
}

// The one url the client is given. It states the container and the audio the
// response will carry, which is the same pair the stream handler reads to
// decide between the file and a remux, so neither end has to guess.
func streamURL(itemID uuid.UUID, source *items.MediaSource, delivered delivery, token, session string, startTicks int64) string {
	query := url.Values{
		"mediaSourceId": {source.ID.String()},
		"container":     {delivered.container},
		"playSessionId": {session},
		"api_key":       {token},
	}
	if delivered.audioCodec != "" {
		query.Set("audioCodec", delivered.audioCodec)
	}
	if startTicks > 0 {
		query.Set("startTimeTicks", strconv.FormatInt(startTicks, 10))
	}
	refuseStreamCopy(query)

	return fmt.Sprintf("/Videos/%s/stream.%s?%s", itemID, delivered.container, query.Encode())
}

// jellyfin-web answers a playback error by asking for the stream all over
// again, and stops only once the url it was handed refuses both stream copies.
// Nothing downstream reads these two: they are what ends the retry.
func refuseStreamCopy(query url.Values) {
	query.Set("allowVideoStreamCopy", "false")
	query.Set("allowAudioStreamCopy", "false")
}
