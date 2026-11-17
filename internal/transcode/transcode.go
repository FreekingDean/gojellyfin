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
	VideoCodec string
	MaxHeight  int32
	Transfer   string
	Threads    int
	Preset     string
}

type format struct {
	muxer       string
	codec       string
	audio       string
	contentType string
	muxerArgs   []string
	video       bool
}

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

const (
	VideoContainer = "mp4"
	VideoCodec     = "h264"
	MaxHeight      = 1080
	maxWidth       = MaxHeight * 16 / 9
	keyframeSecs   = 2
	maxFrameRate   = 30
	TransferPQ     = "smpte2084"
	TransferHLG    = "arib-std-b67"
	Preset         = "veryfast"
	LivePreset     = "ultrafast"
)

const toneMap = "setparams=color_trc=%s:color_primaries=bt2020:colorspace=bt2020nc:range=tv," +
	"zscale=t=linear:npl=100,format=gbrpf32le,zscale=p=bt709," +
	"tonemap=tonemap=hable:desat=0,zscale=t=bt709:m=bt709:r=tv"

func CarriesVideo(container string) bool {
	return formats[strings.ToLower(container)].video
}

func Supported(container string) bool {
	_, ok := formats[strings.ToLower(container)]

	return ok
}

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
	if s.VideoCodec != "" && (!s.Video || !strings.EqualFold(s.VideoCodec, VideoCodec)) {
		return fmt.Errorf("cannot encode video to %q", s.VideoCodec)
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
	switch {
	case s.Video && s.VideoCodec != "":
		args = append(args, "-map", "0:v:0", "-map", "0:a:0")
		args = append(args, s.videoEncode()...)
	case s.Video:
		args = append(args, "-map", "0:v:0", "-map", "0:a:0", "-c:v", "copy")
	default:
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

func (s Spec) preset() string {
	if s.Preset == "" {
		return Preset
	}

	return s.Preset
}

func (s Spec) videoEncode() []string {
	height := int32(MaxHeight)
	if s.MaxHeight > 0 {
		height = min(s.MaxHeight, height)
	}
	width := min(height*16/9, maxWidth)

	filters := []string{"bwdif=mode=send_frame:deint=interlaced"}
	if s.Transfer != "" {
		filters = append(filters, fmt.Sprintf(toneMap, s.Transfer))
	}
	filters = append(filters,
		"scale=w='trunc(iw*sar/2)*2':h=ih",
		"setsar=1",
		fmt.Sprintf("scale=w='min(%d,iw)':h='min(%d,ih)':force_original_aspect_ratio=decrease:force_divisible_by=2", width, height),
		"setsar=1",
		"format=yuv420p",
	)

	args := []string{
		"-vf", strings.Join(filters, ","),
		"-c:v", "libx264",
		"-preset", s.preset(),
		"-crf", "21",
		"-profile:v", "high",
		"-level:v", "4.1",
		"-force_key_frames", fmt.Sprintf("expr:gte(t,n_forced*%d)", keyframeSecs),
		"-sc_threshold", "0",
		"-fpsmax", strconv.Itoa(maxFrameRate),
	}
	if s.Threads > 0 {
		args = append(args, "-threads", strconv.Itoa(s.Threads))
	}

	return args
}
