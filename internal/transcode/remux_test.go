package transcode

import (
	"context"
	"io"
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

func TestRemuxKeepsTheVideoAndReEncodesTheAudio(t *testing.T) {
	mkv := videoSource(t, "rip.mkv", sourceSeconds)

	output, err := start(context.Background(), Spec{
		Path:      mkv,
		Container: VideoContainer,
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

func TestRemuxTargetsAContainerThatCarriesVideo(t *testing.T) {
	if !CarriesVideo(VideoContainer) {
		t.Fatalf("%q carries no video", VideoContainer)
	}

	spec := Spec{Path: "/media/rip.mkv", Container: "mp3", Video: true}
	if err := spec.Valid(); err == nil {
		t.Error("an audio-only container was accepted for a video remux")
	}
}
