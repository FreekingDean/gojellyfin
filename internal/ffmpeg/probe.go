package ffmpeg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"time"
)

// Shorter than the heartbeat window a step is given, so a file the reader
// cannot get through fails the step rather than outliving it: an abandoned step
// is never told to stop, and a read that has wedged has no loop to be told in.
const (
	probeTimeout   = 30 * time.Second
	probeWaitDelay = time.Second
)

type Probe struct {
	Format  Format   `json:"format"`
	Streams []Stream `json:"streams"`
}

type Format struct {
	FormatName string            `json:"format_name"`
	Duration   string            `json:"duration"`
	Size       string            `json:"size"`
	BitRate    string            `json:"bit_rate"`
	Tags       map[string]string `json:"tags"`
}

type Stream struct {
	Index        int               `json:"index"`
	CodecName    string            `json:"codec_name"`
	CodecType    string            `json:"codec_type"`
	Profile      string            `json:"profile"`
	Width        int               `json:"width"`
	Height       int               `json:"height"`
	Channels     int               `json:"channels"`
	SampleRate   string            `json:"sample_rate"`
	BitRate      string            `json:"bit_rate"`
	AvgFrameRate string            `json:"avg_frame_rate"`
	PixelFormat  string            `json:"pix_fmt"`
	Level        int               `json:"level"`
	Disposition  map[string]int    `json:"disposition"`
	Tags         map[string]string `json:"tags"`
}

func (f Format) Seconds() float64 {
	seconds, _ := strconv.ParseFloat(f.Duration, 64)

	return seconds
}

func (f Format) Bytes() int64 {
	size, _ := strconv.ParseInt(f.Size, 10, 64)

	return size
}

func (f Format) Bitrate() int32 {
	bitrate, _ := strconv.Atoi(f.BitRate)

	return int32(bitrate)
}

func (s Stream) Bitrate() int32 {
	bitrate, _ := strconv.Atoi(s.BitRate)

	return int32(bitrate)
}

func (s Stream) SampleRateHz() int32 {
	rate, _ := strconv.Atoi(s.SampleRate)

	return int32(rate)
}

func (s Stream) Language() string {
	return s.Tags["language"]
}

func (s Stream) Title() string {
	return s.Tags["title"]
}

func (s Stream) IsDefault() bool {
	return s.Disposition["default"] == 1
}

func (s Stream) IsForced() bool {
	return s.Disposition["forced"] == 1
}

func ProbeFile(ctx context.Context, path string) (*Probe, error) {
	return probeFile(ctx, path, probeTimeout)
}

func probeFile(ctx context.Context, path string, within time.Duration) (*Probe, error) {
	ctx, cancel := context.WithTimeout(ctx, within)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		path,
	)
	cmd.WaitDelay = probeWaitDelay

	output, err := cmd.Output()
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, fmt.Errorf("ffprobe ran out of time reading %s", path)
		}

		return nil, fmt.Errorf("failed to probe %s: %w", path, err)
	}

	probe := &Probe{}
	if err := json.Unmarshal(output, probe); err != nil {
		return nil, err
	}

	return probe, nil
}

func Available() bool {
	_, err := exec.LookPath("ffprobe")

	return err == nil
}
