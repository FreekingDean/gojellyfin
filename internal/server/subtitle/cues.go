package subtitle

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
)

const ticksPerSecond = 10_000_000

// The formats we can read and write without ffmpeg.
var cueFormats = map[string]bool{"srt": true, "vtt": true}

type window struct {
	start          int64
	end            int64
	copyTimestamps bool
	addTimeMap     bool
}

func (w window) whole() bool {
	return w.start == 0 && w.end == 0 && !w.addTimeMap
}

func (w window) covers(c cue) bool {
	if c.end <= w.start {
		return false
	}

	return w.end == 0 || c.start < w.end
}

func (w window) shift(c cue) cue {
	if w.copyTimestamps {
		return c
	}

	c.start = max(c.start-w.start, 0)
	c.end = max(c.end-w.start, 0)

	return c
}

type cue struct {
	start int64
	end   int64
	lines []string
}

// The file is read as it is written, so a subtitle never lands in memory whole.
func convert(file *os.File, format string, window window) io.ReadCloser {
	reader, writer := io.Pipe()

	go func() {
		defer func() { _ = file.Close() }()
		_ = writer.CloseWithError(writeCues(writer, file, format, window))
	}()

	return reader
}

func writeCues(out io.Writer, in io.Reader, format string, window window) error {
	writer := bufio.NewWriter(out)
	if _, err := writer.WriteString(header(format, window)); err != nil {
		return err
	}

	scanner := bufio.NewScanner(in)
	number := 0
	for {
		next, ok := nextCue(scanner)
		if !ok {
			break
		}
		if !window.covers(next) {
			continue
		}

		number++
		if _, err := writer.WriteString(formatCue(window.shift(next), number, format)); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	return writer.Flush()
}

func header(format string, window window) string {
	if format != "vtt" {
		return ""
	}
	// Tells a HLS player where the segment's cues sit on the media timeline.
	if window.addTimeMap {
		return "WEBVTT\nX-TIMESTAMP-MAP=MPEGTS:900000,LOCAL:00:00:00.000\n\n"
	}

	return "WEBVTT\n\n"
}

func nextCue(scanner *bufio.Scanner) (cue, bool) {
	current := cue{}
	timed := false

	for scanner.Scan() {
		line := strings.TrimSpace(strings.TrimPrefix(scanner.Text(), "\ufeff"))

		if !timed {
			start, end, ok := parseTiming(line)
			if !ok {
				continue
			}
			current.start, current.end, timed = start, end, true

			continue
		}

		if line == "" {
			return current, true
		}
		current.lines = append(current.lines, line)
	}

	return current, timed
}

func parseTiming(line string) (int64, int64, bool) {
	from, to, ok := strings.Cut(line, "-->")
	if !ok {
		return 0, 0, false
	}

	// WebVTT hangs cue settings off the end of the second timestamp.
	fields := strings.Fields(to)
	if len(fields) == 0 {
		return 0, 0, false
	}

	start, ok := parseTimestamp(strings.TrimSpace(from))
	if !ok {
		return 0, 0, false
	}
	end, ok := parseTimestamp(fields[0])
	if !ok {
		return 0, 0, false
	}

	return start, end, true
}

func parseTimestamp(value string) (int64, bool) {
	parts := strings.Split(strings.Replace(value, ",", ".", 1), ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, false
	}

	seconds := float64(0)
	for _, part := range parts {
		unit, err := strconv.ParseFloat(part, 64)
		if err != nil {
			return 0, false
		}
		seconds = seconds*60 + unit
	}

	return int64(math.Round(seconds * ticksPerSecond)), true
}

func formatCue(c cue, number int, format string) string {
	separator, prefix := ".", ""
	if format != "vtt" {
		separator, prefix = ",", strconv.Itoa(number)+"\n"
	}

	return fmt.Sprintf("%s%s --> %s\n%s\n\n",
		prefix,
		formatTimestamp(c.start, separator),
		formatTimestamp(c.end, separator),
		strings.Join(c.lines, "\n"),
	)
}

func formatTimestamp(ticks int64, separator string) string {
	milliseconds := max(ticks, 0) / (ticksPerSecond / 1000)

	return fmt.Sprintf("%02d:%02d:%02d%s%03d",
		milliseconds/3_600_000,
		milliseconds/60_000%60,
		milliseconds/1_000%60,
		separator,
		milliseconds%1_000,
	)
}
