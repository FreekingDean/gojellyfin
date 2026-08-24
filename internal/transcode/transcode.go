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
	Video      bool
	CopyAudio  bool
}

type format struct {
	muxer       string
	codec       string
	audio       string
	contentType string
	muxerArgs   []string
	video       bool
}

// Only containers ffmpeg can write to a non-seekable pipe. Plain mp4 rewrites
// the moov atom on close and so is not one of them; fragmented mp4 is, and it
// is the only container carrying H.264 next to AAC that a browser will play,
// which is what a video remux has to land in. ogg is missing for a second
// reason — it wants libvorbis, which not every ffmpeg build carries.
var formats = map[string]format{
	"mp3":  {muxer: "mp3", codec: "libmp3lame", audio: "mp3", contentType: "audio/mpeg"},
	"aac":  {muxer: "adts", codec: "aac", audio: "aac", contentType: "audio/aac"},
	"ts":   {muxer: "mpegts", codec: "aac", audio: "aac", contentType: "video/mp2t", video: true},
	"opus": {muxer: "opus", codec: "libopus", audio: "opus", contentType: "audio/opus"},
	"mp4": {
		muxer:       "mp4",
		codec:       "aac",
		audio:       "aac",
		contentType: "video/mp4",
		muxerArgs:   []string{"-movflags", "frag_keyframe+empty_moov+default_base_moof"},
		video:       true,
	},
}

// The container a remux lands in when the video is kept as it is and only the
// audio is re-encoded.
const VideoContainer = "mp4"

func CarriesVideo(container string) bool {
	return formats[strings.ToLower(container)].video
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

func AudioCodec(container string) string {
	return formats[strings.ToLower(container)].audio
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
	if s.Video && !CarriesVideo(s.Container) {
		return fmt.Errorf("%q carries no video", s.Container)
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

	args = append(args, "-i", s.Path)
	if s.Video {
		// The video is copied rather than encoded: the browser can already
		// decode it, and only the audio track is unplayable.
		args = append(args, "-map", "0:v:0", "-map", "0:a:0", "-c:v", "copy")
	} else {
		args = append(args, "-vn", "-map", "0:a:0")
	}
	if s.CopyAudio {
		args = append(args, "-c:a", "copy")
	} else {
		args = append(args, "-c:a", target.codec)
		if s.Bitrate > 0 {
			args = append(args, "-b:a", strconv.FormatInt(int64(s.Bitrate), 10))
		}
	}
	args = append(args, target.muxerArgs...)

	return append(args, "-f", target.muxer, "pipe:1")
}
