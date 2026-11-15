package mediainfo

import (
	"net/url"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
)

const nativeAndroid = `{
	"Name": "Jellyfin Android",
	"DirectPlayProfiles": [
		{"Container": "mp4", "Type": "Video", "VideoCodec": "h264,hevc", "AudioCodec": "aac,mp3"},
		{"Container": "mkv", "Type": "Video", "VideoCodec": "h264,hevc", "AudioCodec": "aac,mp3,ac3"}
	],
	"TranscodingProfiles": [
		{"Container": "ts", "Type": "Video", "VideoCodec": "h264", "AudioCodec": "aac,ac3", "Protocol": "hls"}
	]
}`

func TestServer_GetPostedPlaybackInfo_NativeAndroid(t *testing.T) {
	native := profileFrom(t, nativeAndroid)

	t.Run("a file it plays untouched is direct play and carries no transcode", func(t *testing.T) {
		fixture := newFixture(t)
		id := fixture.addRip(t, itemmodel.KindMovie, "mkv", "aac")

		answer := fixture.answer(t, id, firstPlay(&native))
		source := (*answer.MediaSources)[0]

		if !apiutil.Deref(source.SupportsDirectPlay) {
			t.Error("the file was not offered as direct play")
		}
		if apiutil.Deref(source.SupportsTranscoding) || source.TranscodingUrl != nil {
			t.Error("the player was offered a progressive transcode it refuses")
		}
		if source.TranscodingSubProtocol == nil {
			t.Error("TranscodingSubProtocol is missing, and the Kotlin SDK refuses a source without it")
		}
	})

	for _, tc := range []struct {
		name, container, video, audio string
		want                          url.Values
	}{
		{"audio it cannot decode is converted", "mkv", "h264", "dts", url.Values{"audioCodec": {"aac"}}},
		{"a picture it cannot decode is encoded", "mkv", "mpeg2video", "aac", url.Values{"videoCodec": {"h264"}, "maxHeight": {"1080"}}},
	} {
		t.Run(tc.name+" through a direct stream whose session says how", func(t *testing.T) {
			fixture := newFixture(t)
			id := fixture.addRipped(t, itemmodel.KindMovie, tc.container, tc.video, tc.audio)

			answer := fixture.answer(t, id, firstPlay(&native))
			source := (*answer.MediaSources)[0]

			if apiutil.Deref(source.SupportsDirectPlay) || !apiutil.Deref(source.SupportsDirectStream) {
				t.Fatal("a file that has to change was not offered as a direct stream")
			}
			if apiutil.Deref(source.SupportsTranscoding) || source.TranscodingUrl != nil {
				t.Error("the player was offered a progressive transcode it refuses")
			}

			session, err := url.ParseQuery(apiutil.Deref(answer.PlaySessionId))
			if err != nil {
				t.Fatalf("the play session %q does not read back: %v", apiutil.Deref(answer.PlaySessionId), err)
			}
			for key := range tc.want {
				if got := session.Get(key); got != tc.want.Get(key) {
					t.Errorf("session %s = %q, want %q", key, got, tc.want.Get(key))
				}
			}
			if session.Get(sessionKey) == "" {
				t.Error("the play session lost its id")
			}
		})
	}

	t.Run("any other client is still handed one transcode url", func(t *testing.T) {
		fixture := newFixture(t)
		id := fixture.addRip(t, itemmodel.KindMovie, "mkv", "aac")

		web := profileFrom(t, chromeProfile)
		source := fixture.source(t, id, firstPlay(&web))
		if apiutil.Deref(source.SupportsDirectPlay) || source.TranscodingUrl == nil {
			t.Error("a browser was answered the way the native player is")
		}
	})
}

func TestBuildsItsOwnURL(t *testing.T) {
	for name, want := range map[string]bool{
		"Jellyfin Android":                     true,
		"Jellyfin for Android":                 true,
		"Jellyfin for Android External Player": false,
		"Jellyfin Web":                         false,
		"":                                     false,
	} {
		t.Run(name, func(t *testing.T) {
			if got := buildsItsOwnURL(api.DeviceProfile{"Name": name}); got != want {
				t.Errorf("buildsItsOwnURL(%q) = %v, want %v", name, got, want)
			}
		})
	}
}
