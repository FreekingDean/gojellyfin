package worker

import (
	"context"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/ffmpeg"
	"github.com/FreekingDean/gojellyfin/internal/transcode"
)

const sourceSeconds = 3

// A real file rather than a fixture: the point of these tests is that ffmpeg
// decoded something and re-encoded it, which a stub cannot show.
func source(t *testing.T, name string, seconds int) string {
	t.Helper()

	if !Available() {
		t.Fatal("ffmpeg is not on PATH")
	}

	path := filepath.Join(t.TempDir(), name)
	generate := exec.Command("ffmpeg", "-nostdin", "-loglevel", "error",
		"-f", "lavfi",
		"-i", "sine=frequency=440:duration="+strconv.Itoa(seconds),
		path,
	)
	if output, err := generate.CombinedOutput(); err != nil {
		t.Fatalf("failed to generate %q: %v: %s", name, err, output)
	}

	return path
}

func probe(t *testing.T, body []byte, name string) *ffmpeg.Probe {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("failed to write the output: %v", err)
	}

	probed, err := ffmpeg.ProbeFile(context.Background(), path)
	if err != nil {
		t.Fatalf("the output is not decodable: %v", err)
	}

	return probed
}

func TestStartTranscodesToEachSupportedContainer(t *testing.T) {
	flac := source(t, "tone.flac", sourceSeconds)

	for container, codec := range map[string]string{
		"mp3":  "mp3",
		"aac":  "aac",
		"ts":   "aac",
		"opus": "opus",
	} {
		t.Run(container, func(t *testing.T) {
			output, err := start(context.Background(), transcode.Spec{Path: flac, Container: container})
			if err != nil {
				t.Fatalf("failed to start the transcode: %v", err)
			}
			defer func() { _ = output.Close() }()

			body, err := io.ReadAll(output)
			if err != nil {
				t.Fatalf("failed to read the transcode: %v", err)
			}
			if len(body) == 0 {
				t.Fatal("the transcode produced no bytes")
			}

			probed := probe(t, body, "out."+container)
			if len(probed.Streams) == 0 {
				t.Fatal("the output has no streams")
			}
			if got := probed.Streams[0].CodecName; got != codec {
				t.Errorf("the output is %q, want %q", got, codec)
			}
			if seconds := probed.Format.Seconds(); math.Abs(seconds-sourceSeconds) > 1 {
				t.Errorf("the output is %.2fs long, want about %ds", seconds, sourceSeconds)
			}
		})
	}
}

func TestStartSeeksToTheRequestedOffset(t *testing.T) {
	flac := source(t, "tone.flac", sourceSeconds)

	output, err := start(context.Background(), transcode.Spec{
		Path:       flac,
		Container:  "mp3",
		StartTicks: 2 * 10_000_000,
	})
	if err != nil {
		t.Fatalf("failed to start the transcode: %v", err)
	}
	defer func() { _ = output.Close() }()

	body, err := io.ReadAll(output)
	if err != nil {
		t.Fatalf("failed to read the transcode: %v", err)
	}

	if seconds := probe(t, body, "out.mp3").Format.Seconds(); seconds > sourceSeconds-1 {
		t.Errorf("seeking 2s into a %ds source produced %.2fs", sourceSeconds, seconds)
	}
}

// The failure has to arrive as an error rather than as a reader that returns
// nothing, because the caller answers a status on the strength of it.
func TestStartRejectsASourceFfmpegCannotRead(t *testing.T) {
	if !Available() {
		t.Fatal("ffmpeg is not on PATH")
	}

	missing := filepath.Join(t.TempDir(), "absent.flac")
	if _, err := start(context.Background(), transcode.Spec{Path: missing, Container: "mp3"}); err == nil {
		t.Fatal("a missing source started a transcode")
	}
}

func TestStartRejectsAnUnsupportedContainer(t *testing.T) {
	if _, err := start(context.Background(), transcode.Spec{Path: "/dev/null", Container: "mp4"}); err == nil {
		t.Fatal("an unwritable container started a transcode")
	}
}

// Nothing else stops the encode when a client goes away, so a cancelled
// context has to be what takes the process with it.
func TestStartStopsFfmpegWhenTheContextIsCancelled(t *testing.T) {
	flac := source(t, "tone.flac", sourceSeconds)

	ctx, cancel := context.WithCancel(context.Background())
	output, err := start(ctx, transcode.Spec{Path: flac, Container: "mp3"})
	if err != nil {
		t.Fatalf("failed to start the transcode: %v", err)
	}

	process := output.(*process)
	cancel()

	if err := output.Close(); err == nil {
		t.Fatal("the cancelled transcode exited cleanly")
	}
	// A killed process reports Signaled rather than Exited, so a non-nil state
	// is what says it was reaped instead of left behind.
	if process.cmd.ProcessState == nil {
		t.Fatal("ffmpeg is still running")
	}
}
