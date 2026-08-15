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

// The choice the split exists to answer: a browser gets the copy it can decode,
// not the better one it cannot.
func TestPreferredSourceTakesCompatibilityOverQuality(t *testing.T) {
	uhd := source("mkv", "hevc", "eac3", 60_000_000)
	hd := source("mp4", "h264", "aac", 8_000_000)

	browser := Playable{
		Containers:  map[string]bool{"mp4": true},
		AudioCodecs: map[string]bool{"aac": true},
	}

	if got := PreferredSource([]*MediaSource{uhd, hd}, browser); got != hd {
		t.Errorf("chose %s/%s, want the playable copy", got.Container, got.Edges.Streams[0].Codec)
	}
}

// Silence about a kind is silence, so a client that named nothing is not
// refused everything.
func TestPreferredSourceTreatsAnUnstatedKindAsNoRestriction(t *testing.T) {
	uhd := source("mkv", "hevc", "eac3", 60_000_000)
	hd := source("mp4", "h264", "aac", 8_000_000)

	if got := PreferredSource([]*MediaSource{hd, uhd}, Playable{}); got != uhd {
		t.Error("an unrestricted client did not get the best copy")
	}
}

// Nothing playable still hands back a source, so the caller can remux it or
// refuse with a status rather than crashing on a nil.
func TestPreferredSourceFallsBackWhenNothingPlays(t *testing.T) {
	uhd := source("mkv", "hevc", "eac3", 60_000_000)

	refuses := Playable{Containers: map[string]bool{"mp4": true}}
	if got := PreferredSource([]*MediaSource{uhd}, refuses); got != uhd {
		t.Error("nothing playable returned no source at all")
	}
}

// A download is saving bytes rather than decoding them here.
func TestBestSourceIgnoresCompatibility(t *testing.T) {
	uhd := source("mkv", "hevc", "eac3", 60_000_000)
	hd := source("mp4", "h264", "aac", 8_000_000)

	if got := BestSource([]*MediaSource{hd, uhd}); got != uhd {
		t.Error("a download did not get the best copy")
	}
}
