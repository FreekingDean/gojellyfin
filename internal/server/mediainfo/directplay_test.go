package mediainfo

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	streammodal "github.com/FreekingDean/gojellyfin/internal/store/mediastream"
)

const chromeProfile = `{
	"DirectPlayProfiles": [
		{"Container": "webm", "Type": "Video", "VideoCodec": "vp8,vp9,av1", "AudioCodec": "vorbis,opus"},
		{"Container": "mp4,m4v", "Type": "Video", "VideoCodec": "h264,vp8,vp9,av1", "AudioCodec": "aac,mp3,opus,flac,vorbis"},
		{"Container": "mkv", "Type": "Video", "VideoCodec": "h264,vp8,vp9,av1", "AudioCodec": "aac,mp3,opus,flac,vorbis"},
		{"Container": "mp3", "Type": "Audio"},
		{"Container": "m4a", "AudioCodec": "aac", "Type": "Audio"}
	]
}`

func profileFrom(t *testing.T, raw string) api.DeviceProfile {
	t.Helper()

	var profile api.DeviceProfile
	if err := json.Unmarshal([]byte(raw), &profile); err != nil {
		t.Fatalf("failed to read the profile: %v", err)
	}

	return profile
}

func rip(container string, codecs ...string) *items.MediaSource {
	source := &items.MediaSource{Container: container}
	source.Edges.Streams = []*items.MediaStream{{Index: 0, Kind: streammodal.KindVideo, Codec: "h264"}}
	for index, codec := range codecs {
		source.Edges.Streams = append(source.Edges.Streams, &items.MediaStream{
			Index: int32(index + 1),
			Kind:  streammodal.KindAudio,
			Codec: codec,
		})
	}

	return source
}

func TestDirectPlays(t *testing.T) {
	profiles := videoProfiles(profileFrom(t, chromeProfile))

	for _, tc := range []struct {
		name   string
		source *items.MediaSource
		want   bool
	}{
		{name: "an mkv carrying audio the client declared", source: rip("mkv", "aac"), want: true},
		{name: "an mkv carrying audio the client did not declare", source: rip("mkv", "ac3"), want: false},
		{name: "an mkv carrying e-ac-3", source: rip("mkv", "eac3"), want: false},
		{name: "an mp4 carrying audio the client declared", source: rip("mp4", "aac"), want: true},
		{name: "a container the client did not declare", source: rip("avi", "aac"), want: false},
		{name: "the container in another case", source: rip("MKV", "AAC"), want: true},
		{name: "an unprobed source with no streams", source: rip("mkv"), want: true},
		{name: "the first track undecodable and a later one playable", source: rip("mkv", "ac3", "aac"), want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := directPlays(profiles, tc.source); got != tc.want {
				t.Errorf("directPlays = %v, want %v", got, tc.want)
			}
		})
	}

	t.Run("a client that declared nothing is left alone", func(t *testing.T) {
		if !directPlays(videoProfiles(nil), rip("mkv", "ac3")) {
			t.Error("a source was refused to a client with no profile")
		}
	})

	t.Run("audio profiles are not applied to video", func(t *testing.T) {
		if got := len(videoProfiles(profileFrom(t, chromeProfile))); got != 3 {
			t.Errorf("read %d video profiles, want 3", got)
		}
	})
}

func TestTranscodingURL(t *testing.T) {
	item := uuid.New()
	source := &items.MediaSource{ID: uuid.New()}

	raw := transcodingURL(item, source, "the-token", "the-session")

	path, query, found := strings.Cut(raw, "?")
	if !found {
		t.Fatalf("url = %q, want a query", raw)
	}
	if want := "/Videos/" + item.String() + "/stream.mp4"; path != want {
		t.Errorf("path = %q, want %q", path, want)
	}

	values, err := url.ParseQuery(query)
	if err != nil {
		t.Fatalf("failed to read the query: %v", err)
	}
	for name, want := range map[string]string{
		"api_key":       "the-token",
		"container":     "mp4",
		"mediaSourceId": source.ID.String(),
		"playSessionId": "the-session",
	} {
		if got := values.Get(name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
	if values.Has("static") {
		t.Error("the transcoding url asks for the source as it is")
	}
}
