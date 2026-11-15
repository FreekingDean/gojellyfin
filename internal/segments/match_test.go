package segments

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/FreekingDean/gojellyfin/internal/ffmpeg"
)

func episode(t *testing.T, dir string, number int, before float64, bitrate string) string {
	t.Helper()

	path := filepath.Join(dir, fmt.Sprintf("episode%d.mkv", number))
	graph := fmt.Sprintf(
		"anoisesrc=d=%.3f:c=brown:a=0.4:s=%d[cold];"+
			"anoisesrc=d=30:c=pink:a=0.3:s=7,aformat=channel_layouts=mono,volume=1.2[theme];"+
			"anoisesrc=d=60:c=brown:a=0.4:s=%d[story];"+
			"[cold][theme][story]concat=n=3:v=0:a=1",
		before, 100+number, 200+number,
	)
	build := exec.Command("ffmpeg", "-nostdin", "-loglevel", "error",
		"-filter_complex", graph, "-c:a", "aac", "-b:a", bitrate, "-ar", "48000", path)
	if output, err := build.CombinedOutput(); err != nil {
		t.Skipf("this ffmpeg cannot build the fixture: %v: %s", err, output)
	}

	return path
}

func printOf(t *testing.T, path string) []uint32 {
	t.Helper()

	samples, err := ffmpeg.New().DecodeAudio(context.Background(), path, 10*time.Minute)
	if err != nil {
		t.Fatalf("failed to decode %s: %v", path, err)
	}

	return fingerprint(samples)
}

func near(got, want time.Duration) bool {
	return (got - want).Abs() <= 750*time.Millisecond
}

func TestShared(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg is not on PATH")
	}

	dir := t.TempDir()
	first := printOf(t, episode(t, dir, 1, 12.37, "128k"))
	second := printOf(t, episode(t, dir, 2, 41.81, "96k"))

	inFirst, inSecond, ok := shared(first, second)
	if !ok {
		t.Fatal("found no shared intro")
	}

	if !near(inFirst.start, secs(12.37)) || !near(inFirst.end, secs(42.37)) {
		t.Errorf("first episode's intro = %v to %v, want 12.37s to 42.37s", inFirst.start, inFirst.end)
	}
	if !near(inSecond.start, secs(41.81)) || !near(inSecond.end, secs(71.81)) {
		t.Errorf("second episode's intro = %v to %v, want 41.81s to 71.81s", inSecond.start, inSecond.end)
	}
}

func TestShared_NothingInCommon(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg is not on PATH")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "other.mkv")
	build := exec.Command("ffmpeg", "-nostdin", "-loglevel", "error",
		"-f", "lavfi", "-i", "anoisesrc=d=100:c=pink:a=0.3:s=55", "-c:a", "aac", path)
	if output, err := build.CombinedOutput(); err != nil {
		t.Skipf("this ffmpeg cannot build the fixture: %v: %s", err, output)
	}

	if a, b, ok := shared(printOf(t, episode(t, dir, 1, 10, "128k")), printOf(t, path)); ok {
		t.Errorf("found %v and %v shared between unrelated audio", a, b)
	}
}

func secs(seconds float64) time.Duration {
	return time.Duration(seconds * float64(time.Second))
}
