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
// declared is read back out of it rather than bound to a generated type. Each
// entry is a triple, and which of the three fails is the whole of the decision:
// a container costs a mux, an audio codec costs one encode, and a picture
// cannot be encoded at all yet (#481).
type playProfile struct {
	Container  string
	VideoCodec string
	AudioCodec string
	Type       string
}

// What the one url will answer with, which is also what the client is told to
// expect. A codec that matches the source is copied; one that does not is the
// single stream being converted.
type delivery struct {
	container  string
	videoCodec string
	audioCodec string
	refused    bool
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

	container, video, audio := describe(source)
	for _, profile := range profiles {
		if lists(profile.Container, container) && profile.decodes(video, audio) {
			return true
		}
	}

	return false
}

func (p playProfile) decodes(video, audio string) bool {
	return lists(p.VideoCodec, video) && lists(p.AudioCodec, audio)
}

// The cheapest answer that works, in the order Dean asked for it: the file as
// it is; the container changed with both streams copied into it; the audio
// converted with the picture still copied. Converting a picture is #481, so a
// picture nothing declared can carry is refused rather than sent as something
// the client will not decode.
func plan(profiles []playProfile, source *items.MediaSource) delivery {
	container, video, audio := describe(source)

	if directPlays(profiles, source) {
		return copies(container, video, audio)
	}

	open := openTo(profiles, video)
	if len(open) == 0 {
		return delivery{refused: true}
	}

	for _, profile := range open {
		if lists(profile.AudioCodec, audio) {
			return copies(profile.Container, video, audio)
		}
	}

	for _, profile := range open {
		if converted := transcode.AudioCodec(profile.Container); lists(profile.AudioCodec, converted) {
			return copies(profile.Container, video, converted)
		}
	}

	return copies(transcode.VideoContainer, video, transcode.AudioCodec(transcode.VideoContainer))
}

func copies(container, video, audio string) delivery {
	return delivery{container: container, videoCodec: video, audioCodec: audio}
}

// The containers the client declared that carry this picture and that ffmpeg
// can write down a pipe that is never seeked, in the order the client declared
// them. Empty means nothing it named can carry the picture at all.
func openTo(profiles []playProfile, video string) []playProfile {
	open := make([]playProfile, 0, len(profiles))
	for _, profile := range profiles {
		if !lists(profile.VideoCodec, video) {
			continue
		}
		for _, container := range strings.Split(profile.Container, ",") {
			if container = strings.TrimSpace(container); transcode.CarriesVideo(container) {
				open = append(open, playProfile{
					Container:  strings.ToLower(container),
					VideoCodec: profile.VideoCodec,
					AudioCodec: profile.AudioCodec,
				})
			}
		}
	}
	if len(open) == 0 && carries(profiles, video) {
		return []playProfile{{Container: transcode.VideoContainer}}
	}

	return open
}

func carries(profiles []playProfile, video string) bool {
	for _, profile := range profiles {
		if lists(profile.VideoCodec, video) {
			return true
		}
	}

	return false
}

func lists(declared, value string) bool {
	if declared == "" || value == "" {
		return true
	}

	for _, entry := range strings.Split(declared, ",") {
		if strings.EqualFold(strings.TrimSpace(entry), value) {
			return true
		}
	}

	return false
}

// The pair a remux keeps: ffmpeg maps the first stream of each kind, so those
// are the two the client has to be able to decode.
func describe(source *items.MediaSource) (string, string, string) {
	return sourceContainer(source), firstCodec(source, streammodal.KindVideo), firstCodec(source, streammodal.KindAudio)
}

func firstCodec(source *items.MediaSource, kind items.StreamKind) string {
	codec := ""
	index := int32(0)
	for _, stream := range source.Edges.Streams {
		if stream.Kind != kind {
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

// The one url the client is given. It states the container and the two codecs
// the response will carry, which is the same triple the stream handler reads to
// decide what to copy and what to convert, so neither end has to guess.
func streamURL(itemID uuid.UUID, source *items.MediaSource, delivered delivery, token, session string, startTicks int64) string {
	query := url.Values{
		"mediaSourceId": {source.ID.String()},
		"container":     {delivered.container},
		"playSessionId": {session},
		"api_key":       {token},
	}
	if delivered.videoCodec != "" {
		query.Set("videoCodec", delivered.videoCodec)
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
