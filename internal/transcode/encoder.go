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
	slotShares   = 8
	slotWait     = 8 * time.Second
	slotPoll     = 100 * time.Millisecond
)

var (
	ErrBusy         = errors.New("every encode slot is taken")
	ErrNotAvailable = errors.New("ffmpeg is not installed")
	ErrExited       = errors.New("ffmpeg exited")
)

type Encoder struct {
	mu       sync.Mutex
	jobs     int
	taken    int
	stall    time.Duration
	patience time.Duration
}

func NewEncoder(jobs int, stall time.Duration) *Encoder {
	if jobs <= 0 {
		jobs = runtime.NumCPU()
	}
	if stall <= 0 {
		stall = defaultStall
	}

	return &Encoder{jobs: jobs, stall: stall, patience: slotWait}
}

func (e *Encoder) Enabled() bool {
	return Available()
}

func (e *Encoder) Slots() (busy, total int) {
	e.mu.Lock()
	defer e.mu.Unlock()

	return (e.taken + slotShares - 1) / slotShares, e.jobs
}

func (e *Encoder) Stall() time.Duration {
	return e.stall
}

func (e *Encoder) Weight(spec Spec) int {
	if spec.VideoCodec == "" {
		return 0
	}

	return min(VideoSlots, e.jobs)
}

func (e *Encoder) shares(spec Spec) int {
	if weight := e.Weight(spec); weight > 0 {
		return weight * slotShares
	}

	return 1
}

func (e *Encoder) Open(ctx context.Context, spec Spec) (io.ReadCloser, error) {
	if !Available() {
		return nil, ErrNotAvailable
	}

	weight := e.shares(spec)
	if !e.wait(ctx, weight) {
		return nil, ErrBusy
	}
	if spec.VideoCodec != "" {
		spec.Threads = e.Weight(spec)
	}

	output, err := start(ctx, spec)
	if err != nil {
		e.free(weight)

		return nil, err
	}

	return &slot{ReadCloser: output, free: func() { e.free(weight) }}, nil
}

func (e *Encoder) wait(ctx context.Context, weight int) bool {
	deadline := time.NewTimer(e.patience)
	defer deadline.Stop()
	poll := time.NewTicker(slotPoll)
	defer poll.Stop()

	for !e.take(weight) {
		select {
		case <-ctx.Done():
			return false
		case <-deadline.C:
			return false
		case <-poll.C:
		}
	}

	return true
}

func (e *Encoder) take(weight int) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.taken+weight > e.jobs*slotShares {
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
