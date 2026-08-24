package items

import (
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/store"
	streammodal "github.com/FreekingDean/gojellyfin/internal/store/mediastream"
)

func source(container, video, audio string, bitrate int32) *MediaSource {
	record := &MediaSource{Container: container, Bitrate: bitrate}
	record.Edges.Streams = []*store.MediaStream{
		{Kind: streammodal.KindVideo, Codec: video},
		{Kind: streammodal.KindAudio, Codec: audio},
	}

	return record
}

func TestBestSource(t *testing.T) {
	uhd := source("mkv", "hevc", "eac3", 60_000_000)
	hd := source("mp4", "h264", "aac", 8_000_000)

	if got := BestSource([]*MediaSource{hd, uhd}); got != uhd {
		t.Error("a download did not get the best copy")
	}
}
