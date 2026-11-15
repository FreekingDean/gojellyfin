package transcode

import (
	"bytes"
	"context"
	"encoding/binary"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/ffmpeg"
)

const (
	seamSeconds = 40
	seamRate    = 24
)

func seamSource(t *testing.T, gop int, audio ...string) string {
	t.Helper()

	if !Available() {
		t.Fatal("ffmpeg is not on PATH")
	}

	path := filepath.Join(t.TempDir(), "seams.mkv")
	generate := exec.Command("ffmpeg", append(append([]string{"-nostdin", "-loglevel", "error",
		"-f", "lavfi", "-i", "testsrc2=size=320x180:rate=" + strconv.Itoa(seamRate) + ":duration=" + strconv.Itoa(seamSeconds),
		"-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000:duration=" + strconv.Itoa(seamSeconds),
		"-c:v", "libx264", "-preset", "ultrafast", "-bf", "3", "-g", strconv.Itoa(gop), "-keyint_min", strconv.Itoa(gop), "-sc_threshold", "0",
	}, audio...), path)...)
	if output, err := generate.CombinedOutput(); err != nil {
		t.Skipf("this ffmpeg cannot build the fixture: %v: %s", err, output)
	}

	return path
}

func cut(t *testing.T, encoder *Encoder, segment Segment, header bool) []byte {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	output, err := encoder.Segment(ctx, segment)
	if err != nil {
		t.Fatalf("failed to start segment %+v: %v", segment.Window, err)
	}
	defer func() {
		cancel()
		_ = output.Close()
	}()

	var body bytes.Buffer
	if header {
		err = Header(&body, output)
	} else {
		err = Fragments(&body, output, segment.Bounds())
	}
	if err != nil {
		t.Fatalf("failed to read segment %+v: %v", segment.Window, err)
	}

	return body.Bytes()
}

func played(t *testing.T, encoder *Encoder, template Segment, windows []Window) string {
	t.Helper()

	first := template
	first.Window, first.Last = windows[0], len(windows) == 1
	joined := cut(t, encoder, first, true)
	for i, window := range windows {
		segment := template
		segment.Window, segment.Last = window, i == len(windows)-1
		piece := cut(t, encoder, segment, false)
		if len(piece) == 0 {
			t.Fatalf("segment %d (%+v) came back empty", i, window)
		}
		joined = append(joined, piece...)
	}

	path := filepath.Join(t.TempDir(), "joined.mp4")
	if err := os.WriteFile(path, joined, 0o600); err != nil {
		t.Fatal(err)
	}

	decode := exec.Command("ffmpeg", "-nostdin", "-v", "error", "-xerror", "-i", path, "-f", "null", "-")
	if output, err := decode.CombinedOutput(); err != nil || len(bytes.TrimSpace(output)) > 0 {
		t.Errorf("the joined segments do not decode cleanly: %v: %s", err, output)
	}

	return path
}

func timestamps(t *testing.T, path, entries string) [][]float64 {
	t.Helper()

	output, err := exec.Command("ffprobe", "-v", "error", "-show_entries", entries, "-of", "csv=p=0", path).Output()
	if err != nil {
		t.Fatalf("failed to read the timestamps: %v", err)
	}

	var rows [][]float64
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		var row []float64
		for _, field := range strings.Split(strings.Trim(line, ","), ",") {
			value, err := strconv.ParseFloat(field, 64)
			if field == "N/A" {
				value, err = math.NaN(), nil
			}
			if err != nil {
				t.Fatalf("unreadable timestamp %q: %v", line, err)
			}
			row = append(row, value)
		}
		rows = append(rows, row)
	}

	return rows
}

func assertFrames(t *testing.T, path string) {
	t.Helper()

	frames := timestamps(t, path, "frame=pts_time")
	if len(frames) != seamSeconds*seamRate {
		t.Errorf("decoded %d frames, want %d", len(frames), seamSeconds*seamRate)
	}
	step := 1.0 / seamRate
	for i := 1; i < len(frames); i++ {
		if gap := frames[i][0] - frames[i-1][0]; math.Abs(gap-step) > step/4 {
			t.Errorf("frame %d follows frame %d by %.4fs, want %.4fs", i, i-1, gap, step)
		}
	}
}

func assertPackets(t *testing.T, path string) int {
	t.Helper()

	packets := timestamps(t, path, "packet=pts_time,duration_time")
	for i := 1; i < len(packets); i++ {
		if math.IsNaN(packets[i-1][1]) {
			continue
		}
		want := packets[i-1][0] + packets[i-1][1]
		if math.Abs(packets[i][0]-want) > 1e-4 {
			t.Errorf("audio packet %d starts at %.6fs, want %.6fs", i, packets[i][0], want)
		}
	}

	return len(packets)
}

func samples(t *testing.T, path string, args ...string) []int16 {
	t.Helper()

	command := append(append([]string{"-nostdin", "-v", "error"}, args...), "-i", path, "-map", "0:a:0", "-ac", "1", "-ar", "48000", "-f", "s16le", "-")
	output, err := exec.Command("ffmpeg", command...).Output()
	if err != nil {
		t.Fatalf("failed to decode the sound: %v", err)
	}

	decoded := make([]int16, len(output)/2)
	if _, err := binary.Decode(output[:len(decoded)*2], binary.LittleEndian, decoded); err != nil {
		t.Fatal(err)
	}

	return decoded
}

func rms(a, b []int16, offset, from, to int) float64 {
	var sum float64
	for i := from; i < to; i++ {
		difference := float64(a[i]) - float64(b[i+offset])
		sum += difference * difference
	}

	return math.Sqrt(sum / float64(to-from))
}

func TestSegments_CopiedPictureJoinsBackToBack(t *testing.T) {
	source := seamSource(t, 4*seamRate, "-c:a", "ac3")
	keyframes, err := ffmpeg.New().Keyframes(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}

	windows := CopiedWindows(keyframes, seamSeconds*ticksPerSecond)
	if len(windows) != 5 || windows[1].Start != 8*ticksPerSecond {
		t.Fatalf("windows = %+v, want five of eight seconds", windows)
	}

	path := played(t, NewEncoder(4, 0), Segment{Path: source, Track: TrackVideo, Codec: "h264"}, windows)
	assertFrames(t, path)
}

func TestSegments_EncodedPictureJoinsBackToBack(t *testing.T) {
	source := seamSource(t, 10*seamRate, "-c:a", "ac3")

	windows := EncodedWindows(seamSeconds * ticksPerSecond)
	path := played(t, NewEncoder(4, 0), Segment{Path: source, Track: TrackVideo, Encode: true}, windows)
	assertFrames(t, path)

	streams := timestamps(t, path, "stream=height")
	if len(streams) != 1 || streams[0][0] != 180 {
		t.Errorf("encoded heights = %v, want 180", streams)
	}
}

func TestSegments_EncodedSoundJoinsBackToBack(t *testing.T) {
	source := seamSource(t, 4*seamRate, "-c:a", "ac3")

	windows := AudioWindows(seamSeconds * ticksPerSecond)
	path := played(t, NewEncoder(4, 0), Segment{Path: source, Track: TrackAudio, Encode: true}, windows)

	packets := assertPackets(t, path)
	if want := len(windows) * audioSegment; packets < want-audioSegment || packets > want {
		t.Errorf("%d audio packets, want about %d", packets, want)
	}

	joined, reference := samples(t, path), samples(t, source)
	offset, best := 0, math.Inf(1)
	for candidate := -2 * audioFrame; candidate <= 0; candidate++ {
		if fit := rms(joined, reference, candidate, 48000, 49000); fit < best {
			offset, best = candidate, fit
		}
	}

	segment := audioSegment * audioFrame
	for join := segment; join+segment/2 < len(joined) && join+segment/2+offset < len(reference); join += segment {
		across := rms(joined, reference, offset, join-256, join+256)
		within := rms(joined, reference, offset, join-segment/2-256, join-segment/2+256)
		if across > 1.5*within+50 {
			t.Errorf("the sound at the join at sample %d is off by %.0f, against %.0f mid segment", join, across, within)
		}
	}
}

func TestSegments_CopiedSoundJoinsBackToBack(t *testing.T) {
	source := seamSource(t, 4*seamRate, "-c:a", "ac3")

	windows := AudioWindows(seamSeconds * ticksPerSecond)
	path := played(t, NewEncoder(4, 0), Segment{Path: source, Track: TrackAudio, Codec: "ac3"}, windows)

	if packets, want := assertPackets(t, path), seamSeconds*48000/1536; packets < want-1 || packets > want+1 {
		t.Errorf("%d audio packets, want %d", packets, want)
	}
}

func TestSegments_HeaderIsTheSameFromEverySegment(t *testing.T) {
	source := seamSource(t, 4*seamRate, "-c:a", "ac3")
	encoder := NewEncoder(4, 0)

	for name, template := range map[string]Segment{
		"copied picture":  {Path: source, Track: TrackVideo, Codec: "h264"},
		"encoded picture": {Path: source, Track: TrackVideo, Encode: true},
		"encoded sound":   {Path: source, Track: TrackAudio, Encode: true},
		"copied sound":    {Path: source, Track: TrackAudio, Codec: "ac3"},
	} {
		first, later := template, template
		first.Window = Window{Start: 0, End: 8 * ticksPerSecond}
		later.Window = Window{Start: 24 * ticksPerSecond, End: 32 * ticksPerSecond}
		if !bytes.Equal(cut(t, encoder, first, true), cut(t, encoder, later, true)) {
			t.Errorf("%s: the header of a later segment differs from the first", name)
		}
	}
}
