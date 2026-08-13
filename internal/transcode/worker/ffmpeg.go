package worker

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"time"

	"github.com/FreekingDean/gojellyfin/internal/transcode"
)

const stderrLimit = 4096

type process struct {
	cmd    *exec.Cmd
	stdout io.ReadCloser
	output *bufio.Reader
	errors *tail
}

// ffmpeg answers a source it cannot read by exiting rather than by writing a
// short file, so the first byte of output is what separates a transcode that
// started from one that failed. Nothing is reported to the caller until it
// arrives, which keeps a failure from reaching the client as an empty stream.
func start(ctx context.Context, spec transcode.Spec) (io.ReadCloser, error) {
	if err := spec.Valid(); err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, "ffmpeg", spec.Args()...)
	cmd.WaitDelay = time.Second

	errors := &tail{limit: stderrLimit}
	cmd.Stderr = errors

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to pipe ffmpeg output: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	output := bufio.NewReader(stdout)
	if _, err := output.Peek(1); err != nil {
		_ = stdout.Close()
		_ = cmd.Wait()

		return nil, fmt.Errorf("ffmpeg produced no output: %s", errors)
	}

	return &process{cmd: cmd, stdout: stdout, output: output, errors: errors}, nil
}

func (p *process) Read(b []byte) (int, error) {
	return p.output.Read(b)
}

func (p *process) Close() error {
	_ = p.stdout.Close()
	if err := p.cmd.Wait(); err != nil {
		return fmt.Errorf("ffmpeg exited: %w: %s", err, p.errors)
	}

	return nil
}

type tail struct {
	limit int
	seen  []byte
}

func (t *tail) Write(b []byte) (int, error) {
	if room := t.limit - len(t.seen); room > 0 {
		t.seen = append(t.seen, b[:min(room, len(b))]...)
	}

	return len(b), nil
}

func (t *tail) String() string {
	return string(t.seen)
}

func Available() bool {
	_, err := exec.LookPath("ffmpeg")

	return err == nil
}
