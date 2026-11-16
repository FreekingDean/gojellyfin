package mediainfo

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/sessions"
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

	t.Run("a file it plays untouched is direct play", func(t *testing.T) {
		fixture := newFixture(t)
		id := fixture.addRip(t, itemmodel.KindMovie, "mkv", "aac")

		source := fixture.source(t, id, firstPlay(&native))

		if !apiutil.Deref(source.SupportsDirectPlay) {
			t.Error("the file was not offered as direct play, which is the only path the player takes ahead of hls")
		}
		if source.TranscodingSubProtocol == nil {
			t.Error("TranscodingSubProtocol is missing, and the Kotlin SDK refuses a source without it")
		}
	})

	for _, tc := range []struct {
		name, container, video, audio string
		want                          []string
	}{
		{"audio it cannot decode is converted", "mkv", "h264", "dts", []string{"audioCodec=aac"}},
		{"a picture it cannot decode is encoded", "mkv", "mpeg2video", "aac", []string{"videoCodec=h264", "maxHeight=1080"}},
	} {
		t.Run(tc.name+" through hls, the one transcode the player builds", func(t *testing.T) {
			fixture := newFixture(t)
			id := fixture.addRipped(t, itemmodel.KindMovie, tc.container, tc.video, tc.audio)

			source := fixture.source(t, id, firstPlay(&native))

			if apiutil.Deref(source.SupportsDirectPlay) || apiutil.Deref(source.SupportsDirectStream) {
				t.Error("a file that has to change was offered as it is")
			}
			if got := apiutil.Deref(source.TranscodingSubProtocol); got != api.MediaStreamProtocolHls {
				t.Fatalf("transcoding protocol = %q, want hls, since the player closes on any other", got)
			}
			url := apiutil.Deref(source.TranscodingUrl)
			for _, want := range append(tc.want, "/master.m3u8?") {
				if !strings.Contains(url, want) {
					t.Errorf("transcoding url = %q, want it to carry %q", url, want)
				}
			}
		})
	}
}

func TestServer_GetPostedPlaybackInfo_DeviceProfileFallsBackToTheSession(t *testing.T) {
	native := profileFrom(t, nativeAndroid)
	fixture := newFixture(t)
	id := fixture.addRip(t, itemmodel.KindMovie, "mkv", "aac")

	fixture.server.sessions = sessions.New(fixture.client, nil)
	reported := &sessions.Session{ID: uuid.New(), Device: &sessions.Device{Profile: native}}
	ctx := auth.ContextWithSession(auth.ContextWithAuthorization(context.Background(), auth.Authorization{Token: "the-token"}), reported)

	response, err := fixture.server.GetPostedPlaybackInfo(ctx, api.GetPostedPlaybackInfoRequestObject{ItemId: id, JSONBody: firstPlay(nil)})
	if err != nil {
		t.Fatalf("failed to answer playback info: %v", err)
	}
	answered, ok := response.(api.GetPostedPlaybackInfo200JSONResponse)
	if !ok {
		t.Fatalf("playback info answered %T, want a 200", response)
	}

	if source := (*answered.MediaSources)[0]; !apiutil.Deref(source.SupportsDirectPlay) {
		t.Error("a request with no profile was not planned from the profile the device reported")
	}
}
