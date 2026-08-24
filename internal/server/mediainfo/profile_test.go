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

func TestCapabilities(t *testing.T) {
	t.Run("reads the lines a device profile declares for video", func(t *testing.T) {
		can := capabilities(profileFrom(t, chromeProfile))

		if len(can.Profiles) != 3 {
			t.Fatalf("read %d video profiles, want 3 — the audio lines are not video", len(can.Profiles))
		}
		if got := can.Profiles[0].Container; got != "webm" {
			t.Errorf("first container = %q, want webm, in the order the client declared", got)
		}
		if got := can.Profiles[1].AudioCodec; got != "aac,mp3,opus,flac,vorbis" {
			t.Errorf("audio codecs = %q, want them carried across whole", got)
		}
		if got := can.Profiles[1].VideoCodec; got != "h264,vp8,vp9,av1" {
			t.Errorf("video codecs = %q, want them carried across whole", got)
		}
	})

	t.Run("a client that posted no profile declares nothing", func(t *testing.T) {
		if can := capabilities(nil); len(can.Profiles) != 0 {
			t.Errorf("read %d profiles from silence", len(can.Profiles))
		}
	})
}

func planFor(container, audio string) items.Plan {
	source := &items.MediaSource{ID: uuid.New(), Container: "mkv"}
	source.Edges.Streams = []*items.MediaStream{
		{Index: 0, Kind: streammodal.KindVideo, Codec: "h264"},
		{Index: 1, Kind: streammodal.KindAudio, Codec: "ac3"},
	}

	return items.Plan{Source: source, Container: container, AudioCodec: audio}
}

func TestStreamURL(t *testing.T) {
	item := uuid.New()

	t.Run("the suffix names the container the response will carry", func(t *testing.T) {
		plan := planFor("mp4", "aac")

		path, values := splitURL(t, streamURL(item, plan, "the-token", "the-session", 0))

		if want := "/Videos/" + item.String() + "/stream.mp4"; path != want {
			t.Errorf("path = %q, want %q", path, want)
		}
		if got := values.Get("container"); got != "mp4" {
			t.Errorf("container = %q, want mp4", got)
		}
		if got := values.Get("audioCodec"); got != "aac" {
			t.Errorf("audio codec = %q, want aac", got)
		}
	})

	untouched := planFor("mkv", "ac3")

	t.Run("the client is given no way to ask for the source instead", func(t *testing.T) {
		_, values := splitURL(t, streamURL(item, untouched, "the-token", "the-session", 0))

		if values.Has("static") {
			t.Error("the url lets the client ask for the file as it is")
		}
		for name, want := range map[string]string{
			"api_key":       "the-token",
			"playSessionId": "the-session",
			"mediaSourceId": untouched.Source.ID.String(),
		} {
			if got := values.Get(name); got != want {
				t.Errorf("%s = %q, want %q", name, got, want)
			}
		}
	})

	t.Run("a seek carries the position rather than a byte offset", func(t *testing.T) {
		_, values := splitURL(t, streamURL(item, untouched, "t", "s", 25_000_000))

		if got := values.Get("startTimeTicks"); got != "25000000" {
			t.Errorf("start time ticks = %q, want 25000000", got)
		}
	})

	t.Run("playing from the start asks for no position", func(t *testing.T) {
		_, values := splitURL(t, streamURL(item, untouched, "t", "s", 0))

		if values.Has("startTimeTicks") {
			t.Error("the url asks for a position nobody seeked to")
		}
	})

	t.Run("a playback error has somewhere to stop", func(t *testing.T) {
		raw := strings.ToLower(streamURL(item, untouched, "t", "s", 0))

		for _, refusal := range []string{"allowvideostreamcopy=false", "allowaudiostreamcopy=false"} {
			if !strings.Contains(raw, refusal) {
				t.Errorf("url = %q, want it to carry %q so jellyfin-web stops retrying", raw, refusal)
			}
		}
	})
}

func splitURL(t *testing.T, raw string) (string, url.Values) {
	t.Helper()

	path, query, found := strings.Cut(raw, "?")
	if !found {
		t.Fatalf("url = %q, want a query", raw)
	}

	values, err := url.ParseQuery(query)
	if err != nil {
		t.Fatalf("failed to read the query: %v", err)
	}

	return path, values
}
