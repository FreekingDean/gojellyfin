package items

import (
	"testing"

	streammodel "github.com/FreekingDean/gojellyfin/internal/store/mediastream"
)

func source(container, video, audio string, bitrate int32) *MediaSource {
	return &MediaSource{
		Container: container,
		Bitrate:   &bitrate,
		Streams: []*MediaStream{
			{Kind: streammodel.KindVideo, Codec: video},
			{Kind: streammodel.KindAudio, Codec: audio},
		},
	}
}

func TestBestSource(t *testing.T) {
	uhd := source("mkv", "hevc", "eac3", 60_000_000)
	hd := source("mp4", "h264", "aac", 8_000_000)

	if got := BestSource([]*MediaSource{hd, uhd}); got != uhd {
		t.Error("a download did not get the best copy")
	}
}
