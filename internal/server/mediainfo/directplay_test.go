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

// The codec names jellyfin-web actually puts in its DirectPlayProfiles, read
// off the vendored bundle rather than off the documentation, beside what
// ffprobe reports for the same codec. They are the same strings, which is why
// nothing here translates between the two vocabularies.
const webProfile = `{
	"DirectPlayProfiles": [
		{"Container": "webm", "Type": "Video", "VideoCodec": "vp8,vp9,av1", "AudioCodec": "opus"},
		{"Container": "mp4,m4v", "Type": "Video",
			"VideoCodec": "h264,hevc,mpeg2video,vc1,msmpeg4v2,vp9,av1",
			"AudioCodec": "aac,mp3,ac3,eac3,mp2,dca,dts,pcm_s16le,pcm_s24le,truehd,aac_latm,opus,flac,alac,vorbis"}
	]
}`

const streamingStickProfile = `{
	"DirectPlayProfiles": [
		{"Container": "ts", "Type": "Video", "VideoCodec": "h264", "AudioCodec": "aac"}
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

func rip(container string, audio ...string) *items.MediaSource {
	return ripped(container, "h264", audio...)
}

func ripped(container, video string, audio ...string) *items.MediaSource {
	source := &items.MediaSource{ID: uuid.New(), Container: container}
	if video != "" {
		source.Edges.Streams = []*items.MediaStream{{Index: 0, Kind: streammodal.KindVideo, Codec: video}}
	}
	for index, codec := range audio {
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

	t.Run("the names the client writes are the names the probe wrote", func(t *testing.T) {
		declared := videoProfiles(profileFrom(t, webProfile))

		for _, tc := range []struct {
			name   string
			source *items.MediaSource
			want   bool
		}{
			{name: "hevc, which the client spells the same way", source: ripped("mp4", "hevc", "aac"), want: true},
			{name: "mpeg2video, not mpeg2", source: ripped("mp4", "mpeg2video", "aac"), want: true},
			{name: "msmpeg4v2, which is the one the client names", source: ripped("mp4", "msmpeg4v2", "aac"), want: true},
			{name: "vc1", source: ripped("mp4", "vc1", "aac"), want: true},
			{name: "dca, which the client lists beside dts itself", source: ripped("mp4", "h264", "dca"), want: true},
			{name: "eac3, not ec-3", source: ripped("mp4", "h264", "eac3"), want: true},
			{name: "truehd", source: ripped("mp4", "h264", "truehd"), want: true},
			{name: "aac_latm", source: ripped("mp4", "h264", "aac_latm"), want: true},
			{name: "pcm_s16le", source: ripped("mp4", "h264", "pcm_s16le"), want: true},
			{name: "a profile shouting its codec", source: ripped("mp4", "HEVC", "AAC"), want: true},
			{name: "a picture the client never named", source: ripped("mp4", "theora", "aac"), want: false},
			{name: "audio the client never named", source: ripped("mp4", "h264", "wmav2"), want: false},
		} {
			t.Run(tc.name, func(t *testing.T) {
				if got := directPlays(declared, tc.source); got != tc.want {
					t.Errorf("directPlays = %v, want %v", got, tc.want)
				}
			})
		}
	})

	t.Run("a stream nobody probed is never refused for a codec nobody read", func(t *testing.T) {
		declared := videoProfiles(profileFrom(t, webProfile))

		if !directPlays(declared, ripped("mp4", "", "")) {
			t.Error("a source with no probed codecs was refused")
		}
		if !directPlays(videoProfiles(profileFrom(t, `{"DirectPlayProfiles":[{"Container":"mp4","Type":"Video"}]}`)), rip("mp4", "ac3")) {
			t.Error("a profile that named no codecs was read as naming none it takes")
		}
	})

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

func TestPlan(t *testing.T) {
	for _, tc := range []struct {
		name       string
		profile    string
		source     *items.MediaSource
		container  string
		videoCodec string
		audioCodec string
		refused    bool
	}{
		{
			name:       "everything declared is handed over as it is",
			profile:    chromeProfile,
			source:     rip("mkv", "aac"),
			container:  "mkv",
			videoCodec: "h264",
			audioCodec: "aac",
		},
		{
			name:       "codecs declared and the container not costs a mux and nothing else",
			profile:    chromeProfile,
			source:     rip("avi", "aac"),
			container:  "mp4",
			videoCodec: "h264",
			audioCodec: "aac",
		},
		{
			name:       "audio the client cannot decode is the one stream converted",
			profile:    chromeProfile,
			source:     rip("mkv", "ac3"),
			container:  "mp4",
			videoCodec: "h264",
			audioCodec: "aac",
		},
		{
			name:    "a picture nothing declared can carry is refused",
			profile: chromeProfile,
			source:  ripped("mkv", "mpeg4", "aac"),
			refused: true,
		},
		{
			name:    "a picture the real client never names is refused",
			profile: webProfile,
			source:  ripped("mkv", "theora", "aac"),
			refused: true,
		},
		{
			name:       "a client declaring another container gets that one",
			profile:    streamingStickProfile,
			source:     rip("mkv", "ac3"),
			container:  "ts",
			videoCodec: "h264",
			audioCodec: "aac",
		},
		{
			name:       "a client that declared nothing keeps the source",
			source:     rip("mkv", "ac3"),
			container:  "mkv",
			videoCodec: "h264",
			audioCodec: "ac3",
		},
		{
			name:       "an unprobed source is not refused for a codec nobody read",
			profile:    chromeProfile,
			source:     ripped("mkv", ""),
			container:  "mkv",
			videoCodec: "",
			audioCodec: "",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var profiles []playProfile
			if tc.profile != "" {
				profiles = videoProfiles(profileFrom(t, tc.profile))
			}

			delivered := plan(profiles, tc.source)

			if delivered.refused != tc.refused {
				t.Fatalf("refused = %v, want %v", delivered.refused, tc.refused)
			}
			if tc.refused {
				return
			}
			if delivered.container != tc.container {
				t.Errorf("container = %q, want %q", delivered.container, tc.container)
			}
			if delivered.videoCodec != tc.videoCodec {
				t.Errorf("video codec = %q, want %q", delivered.videoCodec, tc.videoCodec)
			}
			if delivered.audioCodec != tc.audioCodec {
				t.Errorf("audio codec = %q, want %q", delivered.audioCodec, tc.audioCodec)
			}
		})
	}

	t.Run("a container the client declared that cannot be written down a pipe is passed over", func(t *testing.T) {
		profiles := videoProfiles(profileFrom(t, `{"DirectPlayProfiles":[
			{"Container": "webm", "Type": "Video", "VideoCodec": "h264", "AudioCodec": "opus"},
			{"Container": "mp4", "Type": "Video", "VideoCodec": "h264", "AudioCodec": "aac"}
		]}`))

		if got := plan(profiles, rip("mkv", "ac3")).container; got != "mp4" {
			t.Errorf("container = %q, want mp4", got)
		}
	})

	t.Run("a picture only an unwritable container declared is still carried", func(t *testing.T) {
		profiles := videoProfiles(profileFrom(t, `{"DirectPlayProfiles":[
			{"Container": "webm", "Type": "Video", "VideoCodec": "vp9", "AudioCodec": "opus"}
		]}`))

		delivered := plan(profiles, ripped("mkv", "vp9", "ac3"))

		if delivered.refused {
			t.Fatal("a picture the client declared was refused because of its container")
		}
		if delivered.container != "mp4" {
			t.Errorf("container = %q, want mp4", delivered.container)
		}
	})
}

func TestStreamURL(t *testing.T) {
	item := uuid.New()
	source := rip("mkv", "ac3")

	t.Run("the suffix names the container the response will carry", func(t *testing.T) {
		delivered := delivery{container: "mp4", audioCodec: "aac"}

		path, values := splitURL(t, streamURL(item, source, delivered, "the-token", "the-session", 0))

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

	t.Run("the client is given no way to ask for the source instead", func(t *testing.T) {
		_, values := splitURL(t, streamURL(item, source, plan(nil, source), "the-token", "the-session", 0))

		if values.Has("static") {
			t.Error("the url lets the client ask for the file as it is")
		}
		for name, want := range map[string]string{
			"api_key":       "the-token",
			"playSessionId": "the-session",
			"mediaSourceId": source.ID.String(),
		} {
			if got := values.Get(name); got != want {
				t.Errorf("%s = %q, want %q", name, got, want)
			}
		}
	})

	t.Run("a seek carries the position rather than a byte offset", func(t *testing.T) {
		_, values := splitURL(t, streamURL(item, source, plan(nil, source), "t", "s", 25_000_000))

		if got := values.Get("startTimeTicks"); got != "25000000" {
			t.Errorf("start time ticks = %q, want 25000000", got)
		}
	})

	t.Run("playing from the start asks for no position", func(t *testing.T) {
		_, values := splitURL(t, streamURL(item, source, plan(nil, source), "t", "s", 0))

		if values.Has("startTimeTicks") {
			t.Error("the url asks for a position nobody seeked to")
		}
	})

	t.Run("a playback error has somewhere to stop", func(t *testing.T) {
		raw := strings.ToLower(streamURL(item, source, plan(nil, source), "t", "s", 0))

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
