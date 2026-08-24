package ffmpeg

import (
	"context"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestProbeFile(t *testing.T) {
	t.Run("gives up on a file that never yields a byte", func(t *testing.T) {
		if !Available() {
			t.Skip("ffprobe is not on PATH")
		}

		path := filepath.Join(t.TempDir(), "wedged.mkv")
		if err := syscall.Mkfifo(path, 0o600); err != nil {
			t.Fatalf("failed to create the pipe: %v", err)
		}

		failed := make(chan error, 1)
		go func() {
			_, err := probeFile(context.Background(), path, time.Second)
			failed <- err
		}()

		select {
		case err := <-failed:
			if err == nil {
				t.Fatal("a file that never yields a byte probed clean")
			}
			if !strings.Contains(err.Error(), path) {
				t.Errorf("error = %q, want the file it gave up on named", err)
			}
		case <-time.After(probeTimeout):
			t.Fatal("ffprobe outlived the bound it was given")
		}
	})
}
