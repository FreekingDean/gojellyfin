package probe

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/ffmpeg"
)

func TestLyricsOf(t *testing.T) {
	t.Run("a tag in the container wins", func(t *testing.T) {
		probe := &ffmpeg.Probe{Format: ffmpeg.Format{Tags: map[string]string{"LYRICS": "[00:01.00]tagged"}}}

		if got := lyricsOf(filepath.Join(t.TempDir(), "song.flac"), probe); got != "[00:01.00]tagged" {
			t.Errorf("lyrics = %q, want the tag", got)
		}
	})

	t.Run("an id3 frame named for its language is a lyric", func(t *testing.T) {
		probe := &ffmpeg.Probe{Streams: []ffmpeg.Stream{{Tags: map[string]string{"lyrics-eng": "sung"}}}}

		if got := lyricsOf(filepath.Join(t.TempDir(), "song.mp3"), probe); got != "sung" {
			t.Errorf("lyrics = %q, want the frame", got)
		}
	})

	t.Run("a lrc beside the file is read when the file carries none", func(t *testing.T) {
		directory := t.TempDir()
		if err := os.WriteFile(filepath.Join(directory, "01 Opener.lrc"), []byte("[00:02.00]beside"), 0o600); err != nil {
			t.Fatal(err)
		}

		if got := lyricsOf(filepath.Join(directory, "01 Opener.flac"), &ffmpeg.Probe{}); got != "[00:02.00]beside" {
			t.Errorf("lyrics = %q, want the sidecar", got)
		}
	})

	t.Run("a file with neither has none", func(t *testing.T) {
		if got := lyricsOf(filepath.Join(t.TempDir(), "song.flac"), &ffmpeg.Probe{}); got != "" {
			t.Errorf("lyrics = %q, want none", got)
		}
	})
}
