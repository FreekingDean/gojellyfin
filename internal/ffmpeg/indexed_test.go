package ffmpeg

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/matroska"
	"github.com/FreekingDean/gojellyfin/internal/observability/tracing"
)

var indexedFixtures = map[string][]string{
	"clip.mkv": {
		"-f", "lavfi", "-i", "testsrc=size=320x240:rate=24000/1001:duration=20",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=20",
		"-c:v", "libx264", "-bf", "3", "-g", "48", "-c:a", "ac3", "-shortest",
	},
	"late.mkv": {
		"-f", "lavfi", "-i", "testsrc=size=320x240:rate=25:duration=12",
		"-c:v", "libx264", "-bf", "2", "-g", "30", "-output_ts_offset", "1.4",
	},
	"clip.webm": {
		"-f", "lavfi", "-i", "testsrc=size=320x240:rate=30:duration=10",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=10",
		"-c:v", "libvpx-vp9", "-deadline", "realtime", "-g", "45", "-c:a", "libopus", "-shortest",
	},
}

func TestKeyframes_IndexedMatchesAFullRead(t *testing.T) {
	ffmpeg := New()
	if ffmpeg.probe == "" {
		t.Skip("ffprobe is not on PATH")
	}

	for name, args := range indexedFixtures {
		t.Run(name, indexedMatches(ffmpeg, name, args))
	}
}

func indexedMatches(ffmpeg *FFMpeg, name string, args []string) func(*testing.T) {
	return func(t *testing.T) {
		path := filepath.Join(t.TempDir(), name)
		build := exec.Command("ffmpeg", append(append([]string{"-nostdin", "-loglevel", "error"}, args...), path)...)
		if output, err := build.CombinedOutput(); err != nil {
			t.Skipf("this ffmpeg cannot build the fixture: %v: %s", err, output)
		}

		indexed, err := ffmpeg.indexed(context.Background(), path)
		if err != nil {
			t.Fatalf("the index was not read: %v", err)
		}
		_, span := tracing.Start(context.Background(), "test")
		read, err := ffmpeg.packets(context.Background(), path, span)
		if err != nil {
			t.Fatal(err)
		}
		if len(read) < 3 || !slices.Equal(indexed, read) {
			t.Errorf("indexed %v, read %v", indexed, read)
		}
	}
}

var unindexedFixtures = map[string]string{
	"clip.mp4":   "mp4",
	"stream.mkv": "matroska",
}

func TestKeyframes_UnindexedFallsBack(t *testing.T) {
	ffmpeg := New()
	if ffmpeg.probe == "" {
		t.Skip("ffprobe is not on PATH")
	}

	for name, format := range unindexedFixtures {
		t.Run(name, fallsBack(ffmpeg, name, format))
	}
}

func fallsBack(ffmpeg *FFMpeg, name, format string) func(*testing.T) {
	return func(t *testing.T) {
		path := filepath.Join(t.TempDir(), name)
		output, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		build := exec.Command("ffmpeg", "-nostdin", "-loglevel", "error",
			"-f", "lavfi", "-i", "testsrc=size=320x240:rate=25:duration=6",
			"-c:v", "libx264", "-g", "50", "-sc_threshold", "0", "-movflags", "frag_keyframe+empty_moov",
			"-f", format, "pipe:1",
		)
		build.Stdout = output
		if err := build.Run(); err != nil {
			t.Skipf("this ffmpeg cannot build the fixture: %v", err)
		}
		_ = output.Close()

		if _, err := ffmpeg.indexed(context.Background(), path); !errors.Is(err, matroska.ErrNoIndex) {
			t.Errorf("indexed a file with no index: %v", err)
		}
		keyframes, err := ffmpeg.Keyframes(context.Background(), path)
		if err != nil || !slices.Equal(keyframes, []int64{0, 2 * ticksPerSecond, 4 * ticksPerSecond}) {
			t.Errorf("keyframes = %v, %v", keyframes, err)
		}
	}
}
