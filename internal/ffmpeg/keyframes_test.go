package ffmpeg

import (
	"context"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestKeyframes(t *testing.T) {
	ffmpeg := New()
	if ffmpeg.probe == "" {
		t.Skip("ffprobe is not on PATH")
	}

	path := filepath.Join(t.TempDir(), "clip.mkv")
	build := exec.Command("ffmpeg", "-nostdin", "-loglevel", "error",
		"-f", "lavfi", "-i", "testsrc=size=320x240:rate=25:duration=6",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=6",
		"-c:v", "libx264", "-bf", "2", "-g", "50", "-sc_threshold", "0", "-c:a", "aac", "-shortest", path,
	)
	if output, err := build.CombinedOutput(); err != nil {
		t.Skipf("this ffmpeg cannot build the fixture: %v: %s", err, output)
	}

	keyframes, err := ffmpeg.Keyframes(context.Background(), path)
	if err != nil {
		t.Fatalf("failed to read the keyframes: %v", err)
	}

	want := []int64{0, 2 * ticksPerSecond, 4 * ticksPerSecond}
	if !slices.Equal(keyframes, want) {
		t.Errorf("keyframes = %v, want %v", keyframes, want)
	}
}

func TestKeyframes_SoundOnly(t *testing.T) {
	ffmpeg := New()
	if ffmpeg.probe == "" {
		t.Skip("ffprobe is not on PATH")
	}

	path := filepath.Join(t.TempDir(), "song.mka")
	build := exec.Command("ffmpeg", "-nostdin", "-loglevel", "error",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=1", "-c:a", "aac", path,
	)
	if output, err := build.CombinedOutput(); err != nil {
		t.Skipf("this ffmpeg cannot build the fixture: %v: %s", err, output)
	}

	keyframes, err := ffmpeg.Keyframes(context.Background(), path)
	if err != nil {
		t.Fatalf("failed to read the keyframes: %v", err)
	}
	if keyframes == nil || len(keyframes) != 0 {
		t.Errorf("keyframes = %#v, want an empty index", keyframes)
	}
}

func TestKeyframe(t *testing.T) {
	cases := map[string]struct {
		ticks int64
		ok    bool
	}{
		"4.000000,K__":  {40_000_000, true},
		"4.041000,___":  {0, false},
		"N/A,K__":       {0, false},
		"1.500000,K_D":  {0, false},
		"0.083000,K__,": {830_000, true},
		"":              {0, false},
	}

	for line, want := range cases {
		ticks, ok := keyframe(line)
		if ticks != want.ticks || ok != want.ok {
			t.Errorf("keyframe(%q) = %d, %v, want %d, %v", line, ticks, ok, want.ticks, want.ok)
		}
	}
}

func TestFromStart(t *testing.T) {
	output := "1.500000,K__,\n1.533000,___,\n5.500000,K__,\n1.500000\n"

	keyframes, err := fromStart(strings.NewReader(output))
	if err != nil {
		t.Fatal(err)
	}
	if want := []int64{0, 4 * ticksPerSecond}; !slices.Equal(keyframes, want) {
		t.Errorf("keyframes = %v, want %v", keyframes, want)
	}
}
