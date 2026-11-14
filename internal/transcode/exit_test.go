package transcode

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fakeFFmpeg(t *testing.T, script string) {
	t.Helper()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ffmpeg"), []byte("#!/bin/sh\n"+script+"\n"), 0o700); err != nil {
		t.Fatalf("failed to write the fake ffmpeg: %v", err)
	}
	t.Setenv("PATH", dir)
}

func TestProcess_Read(t *testing.T) {
	t.Run("fails the read when ffmpeg dies partway", func(t *testing.T) {
		fakeFFmpeg(t, "printf partial; echo 'Error while decoding' >&2; exit 1")

		output, err := start(context.Background(), Spec{Path: "/dev/null", Container: "mp3"})
		if err != nil {
			t.Fatalf("failed to start the transcode: %v", err)
		}

		body, err := io.ReadAll(output)
		if string(body) != "partial" {
			t.Errorf("read %q before the failure, want %q", body, "partial")
		}
		if err == nil {
			t.Fatal("an ffmpeg that exited 1 read as a clean end of stream")
		}
		if !strings.Contains(err.Error(), "Error while decoding") {
			t.Errorf("the error %q does not carry what ffmpeg said", err)
		}
		if closed := output.Close(); closed == nil || closed.Error() != err.Error() {
			t.Errorf("close = %v, want the read's error %v", closed, err)
		}
	})

	t.Run("ends cleanly when ffmpeg exits cleanly", func(t *testing.T) {
		fakeFFmpeg(t, "printf complete")

		output, err := start(context.Background(), Spec{Path: "/dev/null", Container: "mp3"})
		if err != nil {
			t.Fatalf("failed to start the transcode: %v", err)
		}

		body, err := io.ReadAll(output)
		if err != nil {
			t.Fatalf("a clean exit failed the read: %v", err)
		}
		if string(body) != "complete" {
			t.Errorf("read %q, want %q", body, "complete")
		}
		if err := output.Close(); err != nil {
			t.Errorf("close = %v, want nil", err)
		}
	})

	t.Run("does not mistake a pipe closed by ffmpeg for its exit", func(t *testing.T) {
		fakeFFmpeg(t, "printf partial; exec 1>&-; command -p sleep 0.2; exit 3")

		output, err := start(context.Background(), Spec{Path: "/dev/null", Container: "mp3"})
		if err != nil {
			t.Fatalf("failed to start the transcode: %v", err)
		}
		defer func() { _ = output.Close() }()

		var exit interface{ ExitCode() int }
		if _, err := io.ReadAll(output); !errors.As(err, &exit) || exit.ExitCode() != 3 {
			t.Fatalf("read error = %v, want ffmpeg's exit status 3", err)
		}
	})
}
