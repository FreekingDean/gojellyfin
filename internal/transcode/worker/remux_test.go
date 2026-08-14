package worker

import (
	"context"
	"io"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/transcode"
)

// What a rip actually looks like: video a browser decodes next to audio it does
// not. A fixture cannot show that the video survived untouched.
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

func TestRemuxKeepsTheVideoAndReEncodesTheAudio(t *testing.T) {
	mkv := videoSource(t, "rip.mkv", sourceSeconds)

	output, err := start(context.Background(), transcode.Spec{
		Path:      mkv,
		Container: transcode.VideoContainer,
		Video:     true,
	})
	if err != nil {
		t.Fatalf("failed to start the remux: %v", err)
	}
	defer func() { _ = output.Close() }()

	body, err := io.ReadAll(output)
	if err != nil {
		t.Fatalf("failed to read the remux: %v", err)
	}
	if len(body) == 0 {
		t.Fatal("the remux produced no bytes")
	}

	probed := probe(t, body, "out.mp4")

	var video, audio string
	for _, stream := range probed.Streams {
		switch stream.CodecType {
		case "video":
			video = stream.CodecName
		case "audio":
			audio = stream.CodecName
		}
	}

	if video != "h264" {
		t.Errorf("video is %q, want h264 copied through untouched", video)
	}
	if audio != "aac" {
		t.Errorf("audio is %q, want aac — the whole point is that ac3 is gone", audio)
	}
}

// Fragmented mp4 is the only mp4 ffmpeg can write to a pipe, so a plain mp4
// muxer here would fail on close rather than on the first byte.
func TestRemuxTargetsAContainerThatCarriesVideo(t *testing.T) {
	if !transcode.CarriesVideo(transcode.VideoContainer) {
		t.Fatalf("%q carries no video", transcode.VideoContainer)
	}

	spec := transcode.Spec{Path: "/media/rip.mkv", Container: "mp3", Video: true}
	if err := spec.Valid(); err == nil {
		t.Error("an audio-only container was accepted for a video remux")
	}
}
