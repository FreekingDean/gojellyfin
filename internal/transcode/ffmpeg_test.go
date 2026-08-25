package transcode

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
)

const sourceSeconds = 3

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

	prober, err := ffmpeg.New()
	if err != nil {
		t.Fatalf("ffprobe is not on PATH: %v", err)
	}

	probed, err := prober.ProbeFile(context.Background(), path)
	if err != nil {
		t.Fatalf("the output is not decodable: %v", err)
	}

	return probed
}

func TestStart(t *testing.T) {
	t.Run("transcodes to each supported container", func(t *testing.T) {
		flac := source(t, "tone.flac", sourceSeconds)

		for container, codec := range map[string]string{
			"mp3":  "mp3",
			"aac":  "aac",
			"ts":   "aac",
			"opus": "opus",
		} {
			t.Run(container, func(t *testing.T) {
				output, err := start(context.Background(), Spec{Path: flac, Container: container})
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
				if seconds := probed.Format.Duration; math.Abs(seconds-sourceSeconds) > 1 {
					t.Errorf("the output is %.2fs long, want about %ds", seconds, sourceSeconds)
				}
			})
		}
	})

	t.Run("seeks to the requested offset", func(t *testing.T) {
		flac := source(t, "tone.flac", sourceSeconds)

		output, err := start(context.Background(), Spec{
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

		if seconds := probe(t, body, "out.mp3").Format.Duration; seconds > sourceSeconds-1 {
			t.Errorf("seeking 2s into a %ds source produced %.2fs", sourceSeconds, seconds)
		}
	})

	t.Run("rejects a source ffmpeg cannot read", func(t *testing.T) {
		if !Available() {
			t.Fatal("ffmpeg is not on PATH")
		}

		missing := filepath.Join(t.TempDir(), "absent.flac")
		if _, err := start(context.Background(), Spec{Path: missing, Container: "mp3"}); err == nil {
			t.Fatal("a missing source started a transcode")
		}
	})

	t.Run("rejects an unsupported container", func(t *testing.T) {
		if _, err := start(context.Background(), Spec{Path: "/dev/null", Container: "mp4"}); err == nil {
			t.Fatal("an unwritable container started a transcode")
		}
	})

	t.Run("stops ffmpeg when the context is cancelled", func(t *testing.T) {
		flac := source(t, "tone.flac", sourceSeconds)

		ctx, cancel := context.WithCancel(context.Background())
		output, err := start(ctx, Spec{Path: flac, Container: "mp3"})
		if err != nil {
			t.Fatalf("failed to start the transcode: %v", err)
		}

		process := output.(*process)
		cancel()

		if err := output.Close(); err == nil {
			t.Fatal("the cancelled transcode exited cleanly")
		}
		if process.cmd.ProcessState == nil {
			t.Fatal("ffmpeg is still running")
		}
	})

	t.Run("a remux keeps the video and re-encodes the audio", func(t *testing.T) {
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
	})
}
