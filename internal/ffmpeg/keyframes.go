package ffmpeg

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"math"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"time"
)

const (
	keyframeTimeout = 30 * time.Minute
	ticksPerSecond  = 10_000_000
)

func (ffmpeg *FFMpeg) Keyframes(ctx context.Context, path string) ([]int64, error) {
	if ffmpeg.probe == "" {
		return nil, ErrNotAvailable
	}

	ctx, cancel := context.WithTimeout(ctx, keyframeTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, ffmpeg.probe,
		"-v", "error",
		"-select_streams", "V:0",
		"-show_entries", "packet=pts_time,flags",
		"-of", "csv=p=0",
		path,
	)
	cmd.WaitDelay = probeWaitDelay

	output, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start ffprobe: %w", err)
	}

	keyframes := []int64{}
	lines := bufio.NewScanner(output)
	for lines.Scan() {
		if ticks, ok := keyframe(lines.Text()); ok {
			keyframes = append(keyframes, ticks)
		}
	}
	scanned := lines.Err()

	if err := cmd.Wait(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, fmt.Errorf("ffprobe ran out of time reading the keyframes of %s", path)
		}

		return nil, fmt.Errorf("failed to read the keyframes of %s: %w", path, err)
	}
	if scanned != nil {
		return nil, scanned
	}

	slices.Sort(keyframes)

	return slices.Compact(keyframes), nil
}

func keyframe(line string) (int64, bool) {
	pts, flags, found := strings.Cut(strings.TrimSpace(line), ",")
	if !found || !strings.HasPrefix(flags, "K") || strings.Contains(flags, "D") {
		return 0, false
	}

	seconds, err := strconv.ParseFloat(pts, 64)
	if err != nil || math.IsNaN(seconds) || math.IsInf(seconds, 0) {
		return 0, false
	}

	return int64(math.Round(seconds * ticksPerSecond)), true
}
