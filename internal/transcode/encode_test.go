package transcode

import (
	"context"
	"io"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/ffmpeg"
)

func pictureSource(t *testing.T, name, size string, args ...string) string {
	t.Helper()

	if !Available() {
		t.Fatal("ffmpeg is not on PATH")
	}

	path := filepath.Join(t.TempDir(), name)
	generate := exec.Command("ffmpeg", append(append([]string{"-nostdin", "-loglevel", "error",
		"-f", "lavfi", "-i", "testsrc=size=" + size + ":rate=24:duration=" + strconv.Itoa(sourceSeconds),
		"-f", "lavfi", "-i", "sine=frequency=440:duration=" + strconv.Itoa(sourceSeconds),
	}, args...), "-shortest", path)...)
	if output, err := generate.CombinedOutput(); err != nil {
		t.Skipf("this ffmpeg cannot build %q: %v: %s", name, err, output)
	}

	return path
}

func encoded(t *testing.T, spec Spec) *ffmpeg.Probe {
	t.Helper()

	output, err := start(context.Background(), spec)
	if err != nil {
		t.Fatalf("failed to start the encode: %v", err)
	}
	defer func() { _ = output.Close() }()

	body, err := io.ReadAll(output)
	if err != nil {
		t.Fatalf("failed to read the encode: %v", err)
	}

	return probe(t, body, "out."+spec.Container)
}

func streamOf(probed *ffmpeg.Probe, kind string) ffmpeg.Stream {
	for _, stream := range probed.Streams {
		if stream.CodecType == kind {
			return stream
		}
	}

	return ffmpeg.Stream{}
}

func TestStart_VideoEncode(t *testing.T) {
	t.Run("an HDR hevc picture comes out as 1080p SDR h264", func(t *testing.T) {
		uhd := pictureSource(t, "hdr.mkv", "2560x1440",
			"-c:v", "libx265", "-pix_fmt", "yuv420p10le", "-x265-params", "log-level=error",
			"-color_trc", "smpte2084", "-color_primaries", "bt2020", "-colorspace", "bt2020nc",
			"-c:a", "ac3",
		)

		probed := encoded(t, Spec{Path: uhd, Container: VideoContainer, Video: true, VideoCodec: VideoCodec, ToneMap: true})
		video, audio := streamOf(probed, "video"), streamOf(probed, "audio")

		if video.CodecName != "h264" || video.Profile != "High" {
			t.Errorf("video is %s %s, want h264 High", video.CodecName, video.Profile)
		}
		if video.Width != 1920 || video.Height != 1080 {
			t.Errorf("video is %dx%d, want 1920x1080", video.Width, video.Height)
		}
		if video.PixelFormat != "yuv420p" {
			t.Errorf("pixel format is %q, want 8-bit yuv420p", video.PixelFormat)
		}
		if video.ColorTransfer != "bt709" {
			t.Errorf("transfer is %q, want the tone mapped bt709", video.ColorTransfer)
		}
		if video.Level > 41 {
			t.Errorf("level is %v, want at most 4.1", video.Level)
		}
		if audio.CodecName != "aac" {
			t.Errorf("audio is %q, want aac", audio.CodecName)
		}
	})

	t.Run("a picture shorter than 1080p is not scaled up", func(t *testing.T) {
		dvd := pictureSource(t, "dvd.mkv", "720x480", "-c:v", "mpeg4", "-c:a", "aac")

		probed := encoded(t, Spec{Path: dvd, Container: VideoContainer, Video: true, VideoCodec: VideoCodec, CopyAudio: true})
		video := streamOf(probed, "video")

		if video.CodecName != "h264" {
			t.Errorf("video is %q, want h264", video.CodecName)
		}
		if video.Width != 720 || video.Height != 480 {
			t.Errorf("video is %dx%d, want the source's 720x480", video.Width, video.Height)
		}
		if got := streamOf(probed, "audio").CodecName; got != "aac" {
			t.Errorf("audio is %q, want the aac copied", got)
		}
	})

	t.Run("a wider than 16:9 picture is bounded by width", func(t *testing.T) {
		scope := pictureSource(t, "scope.mkv", "2560x1072", "-c:v", "mpeg4", "-c:a", "aac")

		video := streamOf(encoded(t, Spec{Path: scope, Container: "ts", Video: true, VideoCodec: VideoCodec}), "video")

		if video.Width != 1920 || video.Height > 1080 || video.Height%2 != 0 {
			t.Errorf("video is %dx%d, want 1920 wide and an even height under 1080", video.Width, video.Height)
		}
	})

	t.Run("a lower ceiling asked for is honoured", func(t *testing.T) {
		hd := pictureSource(t, "hd.mkv", "1920x1080", "-c:v", "mpeg4", "-c:a", "aac")

		video := streamOf(encoded(t, Spec{Path: hd, Container: VideoContainer, Video: true, VideoCodec: VideoCodec, MaxHeight: 720}), "video")

		if video.Height != 720 {
			t.Errorf("video is %d tall, want 720", video.Height)
		}
	})

	t.Run("a seek starts the encode at the position", func(t *testing.T) {
		hd := pictureSource(t, "seek.mkv", "640x360", "-c:v", "mpeg4", "-c:a", "aac")

		probed := encoded(t, Spec{Path: hd, Container: VideoContainer, Video: true, VideoCodec: VideoCodec, StartTicks: 2 * ticksPerSecond})

		if seconds := probed.Format.Duration; seconds > sourceSeconds-1 {
			t.Errorf("seeking 2s into a %ds source produced %.2fs", sourceSeconds, seconds)
		}
	})
}

func TestSpec_ValidVideoCodec(t *testing.T) {
	for _, tc := range []struct {
		name string
		spec Spec
		ok   bool
	}{
		{name: "h264 into mp4", spec: Spec{Path: "/a.mkv", Container: "mp4", Video: true, VideoCodec: "h264"}, ok: true},
		{name: "a codec nothing here encodes", spec: Spec{Path: "/a.mkv", Container: "mp4", Video: true, VideoCodec: "hevc"}},
		{name: "a picture codec on an audio transcode", spec: Spec{Path: "/a.flac", Container: "mp3", VideoCodec: "h264"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.spec.Valid(); (err == nil) != tc.ok {
				t.Errorf("valid = %v, want %v", err == nil, tc.ok)
			}
		})
	}
}
