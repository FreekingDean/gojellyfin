package ffmpeg

import (
	"context"
	"encoding/json"
	"os/exec"
	"strconv"
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
	cmd := exec.CommandContext(ctx, "ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		path,
	)

	output, err := cmd.Output()
	if err != nil {
		return nil, err
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
