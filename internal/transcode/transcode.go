package transcode

import (
	"fmt"
	"strconv"
	"strings"
)

const ticksPerSecond = 10_000_000

type Spec struct {
	Path       string
	Container  string
	Bitrate    int32
	StartTicks int64
}

type format struct {
	muxer       string
	codec       string
	contentType string
}

// Only containers ffmpeg can write to a non-seekable pipe, which rules out mp4
// and its variants: the moov atom is rewritten on close. ogg is missing for a
// second reason — it wants libvorbis, which not every ffmpeg build carries.
var formats = map[string]format{
	"mp3":  {muxer: "mp3", codec: "libmp3lame", contentType: "audio/mpeg"},
	"aac":  {muxer: "adts", codec: "aac", contentType: "audio/aac"},
	"ts":   {muxer: "mpegts", codec: "aac", contentType: "video/mp2t"},
	"opus": {muxer: "opus", codec: "libopus", contentType: "audio/opus"},
}

func Supported(container string) bool {
	_, ok := formats[strings.ToLower(container)]

	return ok
}

// The client lists what it accepts in preference order, so the first container
// this server can also write is the one both ends agree on.
func Choose(containers []string) string {
	for _, container := range containers {
		if Supported(container) {
			return strings.ToLower(container)
		}
	}

	return ""
}

func ContentType(container string) string {
	return formats[strings.ToLower(container)].contentType
}

func (s Spec) Valid() error {
	if s.Path == "" {
		return fmt.Errorf("no path to transcode")
	}
	if !Supported(s.Container) {
		return fmt.Errorf("cannot transcode to %q", s.Container)
	}

	return nil
}

func (s Spec) Args() []string {
	target := formats[strings.ToLower(s.Container)]

	args := []string{"-nostdin", "-loglevel", "error"}
	if s.StartTicks > 0 {
		seconds := float64(s.StartTicks) / ticksPerSecond
		args = append(args, "-ss", strconv.FormatFloat(seconds, 'f', 3, 64))
	}

	args = append(args,
		"-i", s.Path,
		"-vn",
		"-map", "0:a:0",
		"-c:a", target.codec,
	)
	if s.Bitrate > 0 {
		args = append(args, "-b:a", strconv.FormatInt(int64(s.Bitrate), 10))
	}

	return append(args, "-f", target.muxer, "pipe:1")
}
