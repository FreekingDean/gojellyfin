package ffmpeg

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestProbeFile(t *testing.T) {
	ffmpeg := New()
	if ffmpeg.probe == "" {
		t.Skip("ffprobe is not on PATH")
	}

	t.Run("reads what ffprobe reports about a real file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "clip.mkv")
		build := exec.Command("ffmpeg", "-nostdin", "-loglevel", "error",
			"-f", "lavfi", "-i", "testsrc=size=320x240:rate=30:duration=1",
			"-f", "lavfi", "-i", "sine=frequency=440:duration=1",
			"-c:v", "libx264", "-c:a", "aac", "-disposition:a:0", "default", "-shortest", path,
		)
		if output, err := build.CombinedOutput(); err != nil {
			t.Skipf("this ffmpeg cannot build the fixture: %v: %s", err, output)
		}

		probe, err := ffmpeg.ProbeFile(context.Background(), path)
		if err != nil {
			t.Fatalf("failed to probe the fixture: %v", err)
		}

		if probe.Format.Duration < 0.9 || probe.Format.Duration > 2 {
			t.Errorf("duration = %v, want about a second", probe.Format.Duration)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if probe.Format.Size != info.Size() {
			t.Errorf("size = %d, want %d", probe.Format.Size, info.Size())
		}
		if probe.Format.BitRate <= 0 {
			t.Errorf("bitrate = %d, want the rate ffprobe reports", probe.Format.BitRate)
		}
		if !strings.Contains(probe.Format.FormatName, "matroska") {
			t.Errorf("format = %q, want matroska", probe.Format.FormatName)
		}

		if len(probe.Streams) != 2 {
			t.Fatalf("streams = %d, want a picture and a sound", len(probe.Streams))
		}
		for _, stream := range probe.Streams {
			if stream.Disposition.Forced {
				t.Errorf("%s stream reads as forced", stream.CodecType)
			}
			switch stream.CodecType {
			case "audio":
				if !stream.Disposition.Default {
					t.Error("the audio stream ffmpeg marked default does not read as default")
				}
				if stream.SampleRate != 44100 {
					t.Errorf("sample rate = %d, want 44100", stream.SampleRate)
				}
			case "video":
				if stream.Disposition.Default {
					t.Error("the video stream reads as default when ffmpeg marked only the audio")
				}
				if stream.Width != 320 || stream.Height != 240 {
					t.Errorf("size = %dx%d, want 320x240", stream.Width, stream.Height)
				}
			}
		}
	})

	t.Run("gives up on a file that never yields a byte", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "wedged.mkv")
		if err := syscall.Mkfifo(path, 0o600); err != nil {
			t.Fatalf("failed to create the pipe: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		failed := make(chan error, 1)
		go func() {
			_, err := ffmpeg.ProbeFile(ctx, path)
			failed <- err
		}()

		select {
		case err := <-failed:
			if err == nil {
				t.Fatal("a file that never yields a byte probed clean")
			}
			if !strings.Contains(err.Error(), path) {
				t.Errorf("error = %q, want the file it gave up on named", err)
			}
		case <-time.After(probeTimeout):
			t.Fatal("ffprobe outlived the bound it was given")
		}
	})
}
