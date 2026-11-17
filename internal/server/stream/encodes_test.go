package stream

import (
	"testing"
	"time"
)

func TestEncodes(t *testing.T) {
	t.Run("a session's new picture ends the one it already had and waits for it to go", func(t *testing.T) {
		var running encodes
		stopped := make(chan struct{})
		released := false

		finish := running.begin("token", func() { close(stopped) })
		go func() {
			<-stopped
			time.Sleep(50 * time.Millisecond)
			released = true
			finish()
		}()

		next := running.begin("token", func() {})
		if !released {
			t.Fatal("the second picture began before the first had let go of its slots")
		}
		next()
	})

	t.Run("another session's picture is left alone", func(t *testing.T) {
		var running encodes
		stops := 0

		first := running.begin("one", func() { stops++ })
		second := running.begin("two", func() { stops++ })
		second()
		first()

		if stops != 0 {
			t.Fatalf("stops = %d, want 0", stops)
		}
	})

	t.Run("stopping a session ends its picture and nobody else's", stoppingASessionEndsItsPictureAndNobodyElseS())

	t.Run("a finished picture is forgotten", func(t *testing.T) {
		var running encodes

		running.begin("token", func() { t.Error("stopped a picture that had finished") })()
		running.begin("token", func() {})()
	})
}

func stoppingASessionEndsItsPictureAndNobodyElseS() func(*testing.T) {
	return func(t *testing.T) {
		var running encodes
		stopped := make(chan struct{})

		finish := running.begin("one", func() { close(stopped) })
		other := running.begin("two", func() { t.Error("stopped another session's picture") })
		go func() {
			<-stopped
			finish()
		}()

		running.stop("one")
		running.stop("nobody")
		other()

		select {
		case <-stopped:
		default:
			t.Fatal("the picture was not stopped")
		}
	}
}
