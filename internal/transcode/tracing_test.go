package transcode

import (
	"context"
	"net/http"
	"slices"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/observability/tracing"
)

func TestEncoder_Traced(t *testing.T) {
	traced := func(t *testing.T) (context.Context, *tracing.Span, *tracing.Recorder) {
		t.Helper()

		traces, recorder := tracing.Recorded()
		ctx, span := traces.StartRequest(context.Background(), http.Header{}, "GetVideoStream", http.MethodGet)

		return ctx, span, recorder
	}

	t.Run("an encode is the slot wait and ffmpeg up to its first byte", func(t *testing.T) {
		fakeFFmpeg(t, "printf x; sleep 5")
		ctx, parent, recorder := traced(t)

		output, err := NewEncoder(4, 0).Open(ctx, Spec{Path: "/media/example.mkv", Container: VideoContainer, Video: true, VideoCodec: VideoCodec})
		if err != nil {
			t.Fatal(err)
		}
		names := recorder.Names()
		_ = output.Close()
		parent.End()

		for _, want := range []string{"transcode", "transcode.slot", "ffmpeg"} {
			if !slices.Contains(names, want) {
				t.Errorf("%q had not ended by the first byte: %v", want, names)
			}
		}
		for key, value := range recorder.Values() {
			if value == "/media/example.mkv" {
				t.Errorf("%s carries the file's path", key)
			}
		}
	})

	t.Run("a segment is traced the same way", func(t *testing.T) {
		fakeFFmpeg(t, "printf x")
		ctx, parent, recorder := traced(t)

		output, err := NewEncoder(4, 0).Segment(ctx, Segment{Path: "/media/example.mkv", Track: TrackAudio, Window: Window{Start: 0, End: SegmentTicks}, Encode: true, Codec: "ac3"})
		if err != nil {
			t.Fatal(err)
		}
		_ = output.Close()
		parent.End()

		if names := recorder.Names(); !slices.Contains(names, "transcode.segment") || !slices.Contains(names, "ffmpeg") {
			t.Errorf("spans = %v", names)
		}
		if values := recorder.Values(); values["transcode.track"] != "audio" {
			t.Errorf("track = %q", values["transcode.track"])
		}
	})

}
