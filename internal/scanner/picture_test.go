package scanner

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/ffmpeg"
	streammodal "github.com/FreekingDean/gojellyfin/internal/store/mediastream"
)

func encoded(t *testing.T, name string, args ...string) *ffmpeg.Stream {
	t.Helper()

	if !ffmpeg.Available() {
		t.Skip("ffmpeg is not on PATH")
	}

	path := filepath.Join(t.TempDir(), name)
	build := append([]string{
		"-nostdin", "-loglevel", "error",
		"-f", "lavfi", "-i", "testsrc=size=320x240:rate=15:duration=1",
	}, args...)

	if output, err := exec.Command("ffmpeg", append(build, path)...).CombinedOutput(); err != nil {
		t.Skipf("this ffmpeg cannot build %s: %v: %s", name, err, output)
	}

	probe, err := ffmpeg.ProbeFile(context.Background(), path)
	if err != nil {
		t.Fatalf("failed to probe %s: %v", name, err)
	}

	for _, stream := range probe.Streams {
		if stream.CodecType == "video" {
			return &stream
		}
	}

	t.Fatalf("%s carries no picture", name)

	return nil
}

func TestRangeType(t *testing.T) {
	t.Run("a picture tagged with the pq transfer is HDR10", func(t *testing.T) {
		stream := encoded(t, "hdr.mkv",
			"-vf", "setparams=color_trc=smpte2084:color_primaries=bt2020:colorspace=bt2020nc",
			"-c:v", "libx264", "-pix_fmt", "yuv420p10le",
		)

		if got := rangeType(stream.ColorTransfer); got != streammodal.VideoRangeTypeHDR10 {
			t.Errorf("range = %q from transfer %q, want HDR10", got, stream.ColorTransfer)
		}
	})

	t.Run("a picture tagged with the hlg transfer", func(t *testing.T) {
		stream := encoded(t, "hlg.mkv",
			"-vf", "setparams=color_trc=arib-std-b67:color_primaries=bt2020:colorspace=bt2020nc",
			"-c:v", "libx264", "-pix_fmt", "yuv420p10le",
		)

		if got := rangeType(stream.ColorTransfer); got != streammodal.VideoRangeTypeHLG {
			t.Errorf("range = %q from transfer %q, want HLG", got, stream.ColorTransfer)
		}
	})

	t.Run("a picture tagged with an ordinary transfer is SDR", func(t *testing.T) {
		stream := encoded(t, "sdr.mkv",
			"-vf", "setparams=color_trc=bt709:color_primaries=bt709:colorspace=bt709",
			"-c:v", "libx264", "-pix_fmt", "yuv420p",
		)

		if got := rangeType(stream.ColorTransfer); got != streammodal.VideoRangeTypeSDR {
			t.Errorf("range = %q from transfer %q, want SDR", got, stream.ColorTransfer)
		}
	})

	t.Run("a picture carrying no transfer at all stays unknown", func(t *testing.T) {
		stream := encoded(t, "plain.mkv", "-c:v", "libx264", "-pix_fmt", "yuv420p")

		if stream.ColorTransfer != "" {
			t.Skipf("this ffmpeg tags an untagged encode as %q", stream.ColorTransfer)
		}
		if got := rangeType(stream.ColorTransfer); got != "" {
			t.Errorf("range = %q, want it left unknown rather than called SDR", got)
		}
	})
}

func TestInterlaced(t *testing.T) {
	t.Run("a progressive encode", func(t *testing.T) {
		stream := encoded(t, "progressive.mkv", "-c:v", "libx264", "-pix_fmt", "yuv420p")

		if interlaced(stream.FieldOrder) {
			t.Errorf("field order %q was read as interlaced", stream.FieldOrder)
		}
	})

	t.Run("a top field first encode", func(t *testing.T) {
		stream := encoded(t, "interlaced.mkv",
			"-vf", "setparams=field_mode=tff",
			"-c:v", "libx264", "-pix_fmt", "yuv420p", "-flags", "+ilme+ildct",
		)

		if !interlaced(stream.FieldOrder) {
			t.Errorf("field order %q was not read as interlaced", stream.FieldOrder)
		}
	})
}

func TestAnamorphic(t *testing.T) {
	t.Run("square pixels", func(t *testing.T) {
		stream := encoded(t, "square.mkv", "-c:v", "libx264", "-pix_fmt", "yuv420p")

		if anamorphic(stream.AspectRatio) {
			t.Errorf("sample aspect ratio %q was read as anamorphic", stream.AspectRatio)
		}
	})

	t.Run("stretched pixels", func(t *testing.T) {
		stream := encoded(t, "stretched.mkv",
			"-vf", "setsar=16/11", "-c:v", "libx264", "-pix_fmt", "yuv420p",
		)

		if !anamorphic(stream.AspectRatio) {
			t.Errorf("sample aspect ratio %q was not read as anamorphic", stream.AspectRatio)
		}
	})
}
