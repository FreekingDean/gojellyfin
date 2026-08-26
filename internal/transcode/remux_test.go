package transcode

import (
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
)

func videoSource(t *testing.T, name string, seconds int) string {
	t.Helper()

	if !Available() {
		t.Fatal("ffmpeg is not on PATH")
	}

	path := filepath.Join(t.TempDir(), name)
	generate := exec.Command("ffmpeg", "-nostdin", "-loglevel", "error",
		"-f", "lavfi", "-i", "testsrc=size=320x240:rate=15:duration="+strconv.Itoa(seconds),
		"-f", "lavfi", "-i", "sine=frequency=440:duration="+strconv.Itoa(seconds),
		"-c:v", "libx264", "-pix_fmt", "yuv420p",
		"-c:a", "ac3",
		"-shortest",
		path,
	)
	if output, err := generate.CombinedOutput(); err != nil {
		t.Skipf("this ffmpeg cannot build an h264/ac3 source: %v: %s", err, output)
	}

	return path
}

func TestSpec_Valid(t *testing.T) {
	if !CarriesVideo(VideoContainer) {
		t.Fatalf("%q carries no video", VideoContainer)
	}

	spec := Spec{Path: "/media/rip.mkv", Container: "mp3", Video: true}
	if err := spec.Valid(); err == nil {
		t.Error("an audio-only container was accepted for a video remux")
	}
}
