package ffmpeg

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"time"
)

const (
	AudioSampleRate = 8000
	decodeTimeout   = 90 * time.Second
)

func (ffmpeg *FFMpeg) DecodeAudio(ctx context.Context, path string, length time.Duration) ([]int16, error) {
	if ffmpeg.decode == "" {
		return nil, ErrNotAvailable
	}

	ctx, cancel := context.WithTimeout(ctx, decodeTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, ffmpeg.decode,
		"-nostdin", "-v", "error",
		"-t", strconv.FormatFloat(length.Seconds(), 'f', 3, 64),
		"-i", path,
		"-map", "0:a:0",
		"-ac", "1",
		"-ar", strconv.Itoa(AudioSampleRate),
		"-f", "s16le",
		"-",
	)
	cmd.WaitDelay = probeWaitDelay

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	output, err := cmd.Output()
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, fmt.Errorf("ffmpeg ran out of time decoding %s", path)
		}

		return nil, fmt.Errorf("failed to decode the audio of %s: %w: %s", path, err, bytes.TrimSpace(stderr.Bytes()))
	}

	samples := make([]int16, len(output)/2)
	if _, err := binary.Decode(output[:len(samples)*2], binary.LittleEndian, samples); err != nil {
		return nil, err
	}

	return samples, nil
}
