package ffmpeg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
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
	Duration   float64           `json:"duration,string"`
	Size       int64             `json:"size,string"`
	BitRate    int32             `json:"bit_rate,string"`
	Tags       map[string]string `json:"tags"`
}

type Stream struct {
	Index         int32             `json:"index"`
	CodecName     string            `json:"codec_name"`
	CodecType     string            `json:"codec_type"`
	Profile       string            `json:"profile"`
	Width         int32             `json:"width"`
	Height        int32             `json:"height"`
	Channels      int32             `json:"channels"`
	SampleRate    int32             `json:"sample_rate,string"`
	BitRate       int32             `json:"bit_rate,string"`
	PixelFormat   string            `json:"pix_fmt"`
	ColorTransfer string            `json:"color_transfer"`
	FieldOrder    string            `json:"field_order"`
	AspectRatio   string            `json:"sample_aspect_ratio"`
	Level         float64           `json:"level"`
	Disposition   Disposition       `json:"disposition"`
	Tags          map[string]string `json:"tags"`
}

type Disposition struct {
	Default bool `json:"default"`
	Forced  bool `json:"forced"`
}

func (d *Disposition) UnmarshalJSON(data []byte) error {
	var raw map[string]int
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	d.Default = raw["default"] == 1
	d.Forced = raw["forced"] == 1

	return nil
}

func (ffmpeg *FFMpeg) ProbeFile(ctx context.Context, path string) (*Probe, error) {
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, ffmpeg.probe,
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
