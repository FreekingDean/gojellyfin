package transcode

import (
	"context"
	"errors"
	"io"
	"slices"
	"testing"
	"time"
)

func TestEncoder_Open(t *testing.T) {
	audio := Spec{Path: "/dev/null", Container: "mp3"}
	picture := Spec{Path: "/dev/null", Container: VideoContainer, Video: true, VideoCodec: VideoCodec}

	open := func(t *testing.T, encoder *Encoder, spec Spec) io.ReadCloser {
		t.Helper()

		output, err := encoder.Open(context.Background(), spec)
		if err != nil {
			t.Fatalf("failed to open %+v: %v", spec, err)
		}
		t.Cleanup(func() { _ = output.Close() })

		return output
	}
	refused := func(t *testing.T, encoder *Encoder, spec Spec) {
		t.Helper()

		encoder.patience = 20 * time.Millisecond
		if output, err := encoder.Open(context.Background(), spec); !errors.Is(err, ErrBusy) {
			if output != nil {
				_ = output.Close()
			}
			t.Fatalf("open %+v = %v, want ErrBusy", spec, err)
		}
	}

	t.Run("a picture encode takes several slots and an audio encode one", func(t *testing.T) {
		fakeFFmpeg(t, "printf x")
		encoder := NewEncoder(4, 0)

		video := open(t, encoder, picture)
		refused(t, encoder, picture)
		open(t, encoder, audio)
		refused(t, encoder, audio)

		if err := video.Close(); err != nil {
			t.Fatalf("failed to close the picture encode: %v", err)
		}
		open(t, encoder, picture)
	})

	t.Run("a slot freed while an open waits is taken rather than refused", func(t *testing.T) {
		fakeFFmpeg(t, "printf x")
		encoder := NewEncoder(1, 0)

		held := open(t, encoder, audio)
		time.AfterFunc(150*time.Millisecond, func() { _ = held.Close() })

		open(t, encoder, audio)
	})

	t.Run("a pod with fewer slots than a picture weighs still takes one", func(t *testing.T) {
		fakeFFmpeg(t, "printf x")
		encoder := NewEncoder(1, 0)

		open(t, encoder, picture)
		refused(t, encoder, audio)
	})

	t.Run("an encode that fails to start gives its slots back", func(t *testing.T) {
		fakeFFmpeg(t, "exit 1")
		encoder := NewEncoder(VideoSlots, 0)

		if _, err := encoder.Open(context.Background(), picture); err == nil || errors.Is(err, ErrBusy) {
			t.Fatalf("open = %v, want ffmpeg's failure", err)
		}

		fakeFFmpeg(t, "printf x")
		open(t, encoder, picture)
	})

	t.Run("a closed slot is freed once", func(t *testing.T) {
		fakeFFmpeg(t, "printf x")
		encoder := NewEncoder(VideoSlots+1, 0)

		video := open(t, encoder, picture)
		_ = video.Close()
		_ = video.Close()
		open(t, encoder, picture)
		open(t, encoder, audio)
		refused(t, encoder, audio)
	})
}

func TestSpec_ArgsThreads(t *testing.T) {
	spec := Spec{Path: "in.mkv", Container: VideoContainer, Video: true, VideoCodec: VideoCodec, Threads: VideoSlots}
	args := spec.Args()

	at := slices.Index(args, "-threads")
	if at < 0 || args[at+1] != "3" {
		t.Errorf("args %v do not pin the encode to %d threads", args, VideoSlots)
	}
	if slices.Contains(Spec{Path: "in.mkv", Container: VideoContainer, Video: true, VideoCodec: VideoCodec}.Args(), "-threads") {
		t.Error("an encode with no thread count named one")
	}
}
