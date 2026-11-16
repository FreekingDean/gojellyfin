package ffmpeg

import (
	"context"
	"net/http"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/FreekingDean/gojellyfin/internal/observability/tracing"
)

func TestTraced(t *testing.T) {
	ffmpeg := New()
	if ffmpeg.probe == "" || ffmpeg.decode == "" {
		t.Skip("ffmpeg is not on PATH")
	}

	path := filepath.Join(t.TempDir(), "clip.mkv")
	build := exec.Command("ffmpeg", "-nostdin", "-loglevel", "error",
		"-f", "lavfi", "-i", "testsrc=size=320x240:rate=30:duration=1",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=1",
		"-c:v", "libx264", "-c:a", "aac", "-shortest", path,
	)
	if output, err := build.CombinedOutput(); err != nil {
		t.Skipf("this ffmpeg cannot build the fixture: %v: %s", err, output)
	}

	traces, recorder := tracing.Recorded()
	ctx, parent := traces.StartRequest(context.Background(), http.Header{}, "ProbeFile", http.MethodGet)

	if _, err := ffmpeg.ProbeFile(ctx, path); err != nil {
		t.Fatal(err)
	}
	if _, err := ffmpeg.Keyframes(ctx, path); err != nil {
		t.Fatal(err)
	}
	if _, err := ffmpeg.DecodeAudio(ctx, path, time.Second); err != nil {
		t.Fatal(err)
	}
	if _, err := ffmpeg.ProbeFile(ctx, filepath.Join(t.TempDir(), "missing.mkv")); err == nil {
		t.Fatal("probed a file that is not there")
	}
	parent.End()

	names := recorder.Names()
	for _, want := range []string{"ffprobe", "ffprobe keyframes", "ffmpeg decode audio"} {
		if !slices.Contains(names, want) {
			t.Errorf("no %q span among %v", want, names)
		}
	}
	for key, value := range recorder.Values() {
		if value == path {
			t.Errorf("%s carries the file's path", key)
		}
	}
}
