package ffmpeg

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/FreekingDean/gojellyfin/internal/matroska"
	"github.com/FreekingDean/gojellyfin/internal/observability/tracing"
)

const (
	keyframeTimeout = 30 * time.Minute
	ticksPerSecond  = 10_000_000
)

func (ffmpeg *FFMpeg) Keyframes(ctx context.Context, path string) ([]int64, error) {
	if ffmpeg.probe == "" {
		return nil, ErrNotAvailable
	}

	ctx, span := tracing.Start(ctx, "ffprobe keyframes")
	defer span.End()

	if keyframes, err := ffmpeg.indexed(ctx, path); err == nil {
		span.Set(tracing.Bool("ffmpeg.keyframes_indexed", true), tracing.Int("ffmpeg.keyframes", len(keyframes)))

		return keyframes, nil
	}

	return ffmpeg.packets(ctx, path, span)
}

func (ffmpeg *FFMpeg) packets(ctx context.Context, path string, span *tracing.Span) ([]int64, error) {
	ctx, cancel := context.WithTimeout(ctx, keyframeTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, ffmpeg.probe,
		"-v", "error",
		"-select_streams", "V:0",
		"-show_entries", "packet=pts_time,flags:stream=start_time",
		"-of", "csv=p=0",
		path,
	)
	cmd.WaitDelay = probeWaitDelay

	output, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		span.Failed()

		return nil, fmt.Errorf("failed to start ffprobe: %w", err)
	}

	keyframes, scanned := fromStart(output)

	if err := cmd.Wait(); err != nil {
		span.Failed()
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			span.Set(tracing.Bool("ffmpeg.timed_out", true))

			return nil, fmt.Errorf("ffprobe ran out of time reading the keyframes of %s", path)
		}

		return nil, fmt.Errorf("failed to read the keyframes of %s: %w", path, err)
	}
	if scanned != nil {
		span.Failed()

		return nil, scanned
	}

	span.Set(tracing.Int("ffmpeg.keyframes", len(keyframes)))

	return keyframes, nil
}

func (ffmpeg *FFMpeg) indexed(ctx context.Context, path string) ([]int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	cues, err := matroska.Keyframes(file)
	if err != nil {
		return nil, err
	}

	start, err := ffmpeg.videoStart(ctx, path)
	if err != nil {
		return nil, err
	}

	keyframes := make([]int64, 0, len(cues))
	for _, cue := range cues {
		keyframes = append(keyframes, int64(cue/100)-start)
	}

	return keyframes, nil
}

func (ffmpeg *FFMpeg) videoStart(ctx context.Context, path string) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, ffmpeg.probe,
		"-v", "error",
		"-select_streams", "V:0",
		"-show_entries", "stream=start_time",
		"-of", "csv=p=0",
		path,
	)
	cmd.WaitDelay = probeWaitDelay

	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("failed to read the start of %s: %w", path, err)
	}
	start, ok := toTicks(strings.TrimSpace(string(output)))
	if !ok {
		return 0, fmt.Errorf("ffprobe gave %s no start time", path)
	}

	return start, nil
}

func fromStart(output io.Reader) ([]int64, error) {
	keyframes := []int64{}
	var start int64
	lines := bufio.NewScanner(output)
	for lines.Scan() {
		if ticks, ok := keyframe(lines.Text()); ok {
			keyframes = append(keyframes, ticks)
		} else if ticks, ok := startTime(lines.Text()); ok {
			start = ticks
		}
	}

	for i := range keyframes {
		keyframes[i] -= start
	}
	slices.Sort(keyframes)

	return slices.Compact(keyframes), lines.Err()
}

func keyframe(line string) (int64, bool) {
	pts, flags, found := strings.Cut(strings.TrimSpace(line), ",")
	if !found || !strings.HasPrefix(flags, "K") || strings.Contains(flags, "D") {
		return 0, false
	}

	return toTicks(pts)
}

func startTime(line string) (int64, bool) {
	line = strings.TrimSpace(line)
	if strings.Contains(line, ",") {
		return 0, false
	}

	return toTicks(line)
}

func toTicks(text string) (int64, bool) {
	seconds, err := strconv.ParseFloat(text, 64)
	if err != nil || math.IsNaN(seconds) || math.IsInf(seconds, 0) {
		return 0, false
	}

	return int64(math.Round(seconds * ticksPerSecond)), true
}
