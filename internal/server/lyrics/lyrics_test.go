package lyrics

import (
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/lyrics"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

func TestLyricDto(t *testing.T) {
	converted := lyricDto(lyrics.Parse("[ar:The Band]\n[00:01.00]hello"))

	if !apiutil.Deref(converted.Metadata.IsSynced) {
		t.Error("synced lyrics were answered as unsynced")
	}
	if got := apiutil.Deref(converted.Metadata.Artist); got != "The Band" {
		t.Errorf("artist = %q, want the tag", got)
	}
	if converted.Metadata.Album != nil {
		t.Errorf("album = %q, want nothing for a tag the file lacks", *converted.Metadata.Album)
	}
	lines := *converted.Lyrics
	if len(lines) != 1 || apiutil.Deref(lines[0].Start) != 10_000_000 || apiutil.Deref(lines[0].Text) != "hello" {
		t.Errorf("lines = %+v, want the one timed line", lines)
	}
}
