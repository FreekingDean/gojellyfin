package transcode

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
)

type Track int

const (
	TrackVideo Track = iota
	TrackAudio
)

const (
	SegmentTicks      = 6 * ticksPerSecond
	AudioRate         = 48000
	audioFrame        = 1024
	audioSegment      = 282
	AudioSegmentTicks = audioSegment * audioFrame * ticksPerSecond / AudioRate
	audioPreroll      = 2 * audioFrame
	seekLead          = 3 * ticksPerSecond
	readPast          = ticksPerSecond / 2
	AudioChannels     = 2
	AudioBitrate      = 192_000
	SegmentAudioCodec = "aac"
	tolerance         = ticksPerSecond / 1000
	fragmentMicros    = 1_000_000
)

var copiedVideo = []string{"h264", "hevc", "av1", "vp9"}

var copiedAudio = []string{"aac", "ac3", "eac3", "mp3", "opus", "flac", "alac"}

func CopiesVideo(codec string) bool {
	return slices.Contains(copiedVideo, strings.ToLower(codec))
}

func CopiesAudio(codec string) bool {
	return slices.Contains(copiedAudio, strings.ToLower(codec))
}

type Window struct {
	Start int64
	End   int64
}

func (w Window) Length() int64 {
	return w.End - w.Start
}

func CopiedWindows(keyframes []int64, runtime int64) []Window {
	if len(keyframes) == 0 {
		return nil
	}

	var windows []Window
	start := keyframes[0]
	for _, keyframe := range keyframes[1:] {
		if keyframe-start >= SegmentTicks && keyframe < runtime {
			windows = append(windows, Window{Start: start, End: keyframe})
			start = keyframe
		}
	}

	return append(windows, Window{Start: start, End: max(runtime, start+1)})
}

func EncodedWindows(runtime int64) []Window {
	return grid(runtime, SegmentTicks)
}

func AudioWindows(runtime int64) []Window {
	return grid(runtime, AudioSegmentTicks)
}

func grid(runtime, length int64) []Window {
	var windows []Window
	for start := int64(0); start < runtime || start == 0; start += length {
		windows = append(windows, Window{Start: start, End: min(start+length, max(runtime, start+1))})
	}

	return windows
}

type Segment struct {
	Path      string
	Track     Track
	Window    Window
	Last      bool
	Encode    bool
	Codec     string
	Transfer  string
	MaxHeight int32
	Threads   int
}

func (s Segment) Valid() error {
	if s.Path == "" {
		return fmt.Errorf("no path to cut a segment from")
	}
	if s.Window.End <= s.Window.Start || s.Window.Start < 0 {
		return fmt.Errorf("segment window %d-%d is empty", s.Window.Start, s.Window.End)
	}

	switch {
	case s.Track == TrackVideo && !s.Encode && !CopiesVideo(s.Codec):
		return fmt.Errorf("cannot copy %q video into a segment", s.Codec)
	case s.Track == TrackAudio && !s.Encode && !CopiesAudio(s.Codec):
		return fmt.Errorf("cannot copy %q audio into a segment", s.Codec)
	}

	return nil
}

func (s Segment) Bounds() Window {
	if s.Last {
		return Window{Start: s.Window.Start, End: math.MaxInt64}
	}

	return s.Window
}

func (s Segment) EncodesVideo() bool {
	return s.Track == TrackVideo && s.Encode
}

func (s Segment) ContentType() string {
	if s.Track == TrackAudio {
		return "audio/mp4"
	}

	return "video/mp4"
}

func (s Segment) Args() []string {
	args := []string{"-nostdin", "-loglevel", "error", "-copyts"}
	if s.Track == TrackAudio && s.Encode {
		args = append(args, "-noaccurate_seek")
	}
	if start := s.seek(); start > 0 {
		args = append(args, "-ss", seconds(start))
	}
	args = append(args, "-i", s.Path)

	if s.Track == TrackVideo {
		args = append(args, s.videoArgs()...)
	} else {
		args = append(args, s.audioArgs()...)
	}

	return append(args,
		"-map_metadata", "-1",
		"-map_chapters", "-1",
		"-avoid_negative_ts", "disabled",
		"-strict", "experimental",
		"-f", "mp4",
		"-movflags", s.movflags(),
		"pipe:1",
	)
}

func (s Segment) movflags() string {
	flags := "frag_keyframe+empty_moov+default_base_moof+frag_discont+negative_cts_offsets"
	if s.Track == TrackAudio {
		flags += "+delay_moov"
	}

	return flags
}

func (s Segment) seek() int64 {
	if s.Track == TrackVideo {
		return s.Window.Start
	}

	return max(0, s.Window.Start-seekLead)
}

func (s Segment) until() []string {
	if s.Last {
		return nil
	}

	end := s.Window.End
	if !s.EncodesVideo() {
		end += readPast
	}

	return []string{"-to", seconds(end)}
}

func (s Segment) videoArgs() []string {
	args := []string{"-map", "0:v:0"}
	args = append(args, s.until()...)
	if !s.Encode {
		args = append(args, "-c:v", "copy")
		if strings.EqualFold(s.Codec, "hevc") {
			args = append(args, "-tag:v", "hvc1")
		}

		return args
	}

	encode := Spec{MaxHeight: s.MaxHeight, Transfer: s.Transfer, Threads: s.Threads}.videoEncode()
	for i, arg := range encode {
		if arg == "-force_key_frames" {
			encode[i+1] = fmt.Sprintf("expr:gte(t,%s+n_forced*%d)", seconds(s.Window.Start), keyframeSecs)
		}
	}

	return append(args, encode...)
}

func (s Segment) audioArgs() []string {
	args := []string{"-map", "0:a:0", "-vn"}
	args = append(args, s.until()...)
	if s.Encode {
		args = append(args,
			"-af", s.alignment(),
			"-c:a", SegmentAudioCodec,
			"-ac", strconv.Itoa(AudioChannels),
			"-b:a", strconv.Itoa(AudioBitrate),
		)
	} else {
		args = append(args, "-c:a", "copy")
	}

	return append(args, "-bsf:a", s.cut(), "-frag_duration", strconv.Itoa(fragmentMicros))
}

func (s Segment) alignment() string {
	if s.Window.Start == 0 {
		return fmt.Sprintf("aresample=%d:async=1:first_pts=0", AudioRate)
	}

	preroll := s.Window.Start - audioPreroll*ticksPerSecond/AudioRate

	return fmt.Sprintf("aresample=%d,atrim=start=%s", AudioRate, seconds(preroll))
}

func (s Segment) cut() string {
	drop := fmt.Sprintf("lt(pts*tb\\,%s)", seconds(s.Window.Start-tolerance))
	if !s.Last {
		drop += fmt.Sprintf("+gte(pts*tb\\,%s)", seconds(s.Window.End-tolerance))
	}

	return "noise=drop='" + drop + "'"
}

func seconds(ticks int64) string {
	return strconv.FormatFloat(float64(ticks)/ticksPerSecond, 'f', 6, 64)
}
