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

	t.Run("a picture encode takes whole slots and a lighter one a share of one", aPictureEncodeTakesWholeSlotsAndALighterOneAShareOfOne(open, picture, refused, audio))

	t.Run("a pod holds several lighter encodes to a slot", aPodHoldsSeveralLighterEncodesToASlot(open, audio, refused, picture))

	t.Run("a slot freed while an open waits is taken rather than refused", func(t *testing.T) {
		fakeFFmpeg(t, "printf x")
		encoder := NewEncoder(1, 0)

		held := open(t, encoder, picture)
		time.AfterFunc(150*time.Millisecond, func() { _ = held.Close() })

		open(t, encoder, picture)
	})

	t.Run("a pod with fewer slots than a picture weighs still takes one", func(t *testing.T) {
		fakeFFmpeg(t, "printf x")
		encoder := NewEncoder(1, 0)

		open(t, encoder, picture)
		refused(t, encoder, audio)
	})

	t.Run("an encode that fails to start gives its slots back", anEncodeThatFailsToStartGivesItsSlotsBack(picture, open))

	t.Run("a closed slot is freed once", aClosedSlotIsFreedOnce(open, picture, audio, refused))
}

func anEncodeThatFailsToStartGivesItsSlotsBack(picture Spec, open func(t *testing.T, encoder *Encoder, spec Spec) io.ReadCloser) func(*testing.T) {
	return func(t *testing.T) {
		fakeFFmpeg(t, "exit 1")
		encoder := NewEncoder(VideoSlots, 0)

		if _, err := encoder.Open(context.Background(), picture); err == nil || errors.Is(err, ErrBusy) {
			t.Fatalf("open = %v, want ffmpeg's failure", err)
		}

		fakeFFmpeg(t, "printf x")
		open(t, encoder, picture)
	}
}

func aClosedSlotIsFreedOnce(open func(t *testing.T, encoder *Encoder, spec Spec) io.ReadCloser, picture Spec, audio Spec, refused func(t *testing.T, encoder *Encoder, spec Spec)) func(*testing.T) {
	return func(t *testing.T) {
		fakeFFmpeg(t, "printf x")
		encoder := NewEncoder(VideoSlots+1, 0)

		video := open(t, encoder, picture)
		_ = video.Close()
		_ = video.Close()
		open(t, encoder, picture)
		for range SlotShares {
			open(t, encoder, audio)
		}
		refused(t, encoder, audio)
	}
}

func aPodHoldsSeveralLighterEncodesToASlot(open func(t *testing.T, encoder *Encoder, spec Spec) io.ReadCloser, audio Spec, refused func(t *testing.T, encoder *Encoder, spec Spec), picture Spec) func(*testing.T) {
	return func(t *testing.T) {
		fakeFFmpeg(t, "printf x")
		encoder := NewEncoder(2, 0)

		for range 2 * SlotShares {
			open(t, encoder, audio)
		}
		refused(t, encoder, audio)
		refused(t, encoder, picture)

		if busy, total := encoder.Slots(); busy != 2 || total != 2 {
			t.Fatalf("slots = %d of %d, want 2 of 2", busy, total)
		}
	}
}

func aPictureEncodeTakesWholeSlotsAndALighterOneAShareOfOne(open func(t *testing.T, encoder *Encoder, spec Spec) io.ReadCloser, picture Spec, refused func(t *testing.T, encoder *Encoder, spec Spec), audio Spec) func(*testing.T) {
	return func(t *testing.T) {
		fakeFFmpeg(t, "printf x")
		encoder := NewEncoder(4, 0)

		video := open(t, encoder, picture)
		refused(t, encoder, picture)
		for range SlotShares {
			open(t, encoder, audio)
		}
		refused(t, encoder, audio)

		if err := video.Close(); err != nil {
			t.Fatalf("failed to close the picture encode: %v", err)
		}
		open(t, encoder, picture)
	}
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
