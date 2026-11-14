package transcode

import (
	"context"
	"errors"
	"io"
	"runtime"
	"sync"
	"time"
)

const (
	defaultStall = 30 * time.Second
	VideoSlots   = 3
)

var (
	ErrBusy         = errors.New("every encode slot is taken")
	ErrNotAvailable = errors.New("ffmpeg is not installed")
	ErrExited       = errors.New("ffmpeg exited")
)

type Encoder struct {
	mu    sync.Mutex
	jobs  int
	taken int
	stall time.Duration
}

func NewEncoder(jobs int, stall time.Duration) *Encoder {
	if jobs <= 0 {
		jobs = runtime.NumCPU()
	}
	if stall <= 0 {
		stall = defaultStall
	}

	return &Encoder{jobs: jobs, stall: stall}
}

func (e *Encoder) Enabled() bool {
	return Available()
}

func (e *Encoder) Slots() (busy, total int) {
	e.mu.Lock()
	defer e.mu.Unlock()

	return e.taken, e.jobs
}

func (e *Encoder) Stall() time.Duration {
	return e.stall
}

func (e *Encoder) Weight(spec Spec) int {
	if spec.VideoCodec == "" {
		return 1
	}

	return min(VideoSlots, e.jobs)
}

func (e *Encoder) Open(ctx context.Context, spec Spec) (io.ReadCloser, error) {
	if !Available() {
		return nil, ErrNotAvailable
	}

	weight := e.Weight(spec)
	if !e.take(weight) {
		return nil, ErrBusy
	}
	if spec.VideoCodec != "" {
		spec.Threads = weight
	}

	output, err := start(ctx, spec)
	if err != nil {
		e.free(weight)

		return nil, err
	}

	return &slot{ReadCloser: output, free: func() { e.free(weight) }}, nil
}

func (e *Encoder) take(weight int) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.taken+weight > e.jobs {
		return false
	}
	e.taken += weight

	return true
}

func (e *Encoder) free(weight int) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.taken -= weight
}

type slot struct {
	io.ReadCloser
	free  func()
	freed bool
}

func (s *slot) Close() error {
	if !s.freed {
		s.freed = true
		s.free()
	}

	return s.ReadCloser.Close()
}
