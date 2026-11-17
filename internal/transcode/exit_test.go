package transcode

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
		if !errors.Is(err, ErrExited) {
			t.Fatalf("read error = %v, want ErrExited", err)
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

var silentCases = []struct {
	name   string
	script string
	cancel bool
	want   string
}{
	{name: "a clean exit says so", script: "exit 0", want: "exited cleanly without writing anything"},
	{name: "a failed exit carries its status", script: "echo 'No such file' >&2; exit 1", want: "exit status 1: No such file"},
	{name: "a stopped request says it was stopped", script: "exec /bin/sleep 5", cancel: true, want: "stopped before its first byte: context canceled"},
}

func TestRun_NoOutput(t *testing.T) {
	for _, c := range silentCases {
		t.Run(c.name, silentRun(c.script, c.cancel, c.want))
	}
}

func silentRun(script string, cancel bool, want string) func(*testing.T) {
	return func(t *testing.T) {
		fakeFFmpeg(t, script)

		ctx, stop := context.WithCancel(context.Background())
		defer stop()
		if cancel {
			go func() {
				time.Sleep(100 * time.Millisecond)
				stop()
			}()
		}

		_, err := start(ctx, Spec{Path: "/dev/null", Container: "mp3"})
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("error = %v, want one naming %q", err, want)
		}
	}
}
