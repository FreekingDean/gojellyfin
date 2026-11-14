package transcode

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
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

		probed := encoded(t, Spec{Path: uhd, Container: VideoContainer, Video: true, VideoCodec: VideoCodec, Transfer: TransferPQ})
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

func frameRate(t *testing.T, path string) string {
	t.Helper()

	output, err := exec.Command("ffprobe", "-v", "error", "-select_streams", "v:0",
		"-show_entries", "stream=avg_frame_rate", "-of", "default=nw=1:nk=1", path).Output()
	if err != nil {
		t.Fatalf("failed to read the frame rate of %s: %v", path, err)
	}

	return strings.TrimSpace(string(output))
}

func encodedFile(t *testing.T, spec Spec) string {
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

	path := filepath.Join(t.TempDir(), "out."+spec.Container)
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("failed to write the encode: %v", err)
	}

	return path
}

func TestStart_VideoEncodeShape(t *testing.T) {
	t.Run("an anamorphic picture keeps its display shape", func(t *testing.T) {
		pal := pictureSource(t, "pal.mkv", "720x576", "-vf", "setsar=64/45", "-c:v", "mpeg4", "-c:a", "aac")

		video := streamOf(encoded(t, Spec{Path: pal, Container: VideoContainer, Video: true, VideoCodec: VideoCodec}), "video")

		if video.Width != 1024 || video.Height != 576 {
			t.Errorf("video is %dx%d, want the 16:9 1024x576 it displays as", video.Width, video.Height)
		}
		if video.AspectRatio != "1:1" {
			t.Errorf("sample aspect ratio is %q, want square pixels", video.AspectRatio)
		}
	})

	t.Run("a 60fps picture comes out at 30 so level 4.1 holds at 1080p", func(t *testing.T) {
		fast := pictureSource(t, "fast.mkv", "1920x1080", "-r", "60", "-c:v", "mpeg4", "-c:a", "aac")

		if got := frameRate(t, encodedFile(t, Spec{Path: fast, Container: VideoContainer, Video: true, VideoCodec: VideoCodec})); got != "30/1" {
			t.Errorf("frame rate is %s, want 30/1", got)
		}
	})

	t.Run("an interlaced picture is deinterlaced to one frame per frame", func(t *testing.T) {
		interlaced := pictureSource(t, "interlaced.mkv", "1920x1080",
			"-c:v", "mpeg2video", "-flags", "+ilme+ildct", "-top", "1", "-c:a", "aac")

		path := encodedFile(t, Spec{Path: interlaced, Container: VideoContainer, Video: true, VideoCodec: VideoCodec})
		if got := frameRate(t, path); got != "24/1" {
			t.Errorf("frame rate is %s, want the source's 24/1 rather than a field per frame", got)
		}

		probed, err := ffmpeg.New().ProbeFile(context.Background(), path)
		if err != nil {
			t.Fatalf("the output is not decodable: %v", err)
		}
		if order := streamOf(probed, "video").FieldOrder; order != "" && order != "progressive" {
			t.Errorf("field order is %q, want progressive", order)
		}
	})

	t.Run("an untagged picture said to be HDR still tone maps", func(t *testing.T) {
		for _, transfer := range []string{TransferPQ, TransferHLG} {
			untagged := pictureSource(t, "untagged.mkv", "640x360", "-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac")

			video := streamOf(encoded(t, Spec{Path: untagged, Container: VideoContainer, Video: true, VideoCodec: VideoCodec, Transfer: transfer}), "video")
			if video.ColorTransfer != "bt709" {
				t.Errorf("%s: transfer is %q, want bt709", transfer, video.ColorTransfer)
			}
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
