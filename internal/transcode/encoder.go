package transcode

import (
	"context"
	"errors"
	"io"
	"runtime"
	"time"
)

const defaultStall = 30 * time.Second

var (
	ErrBusy         = errors.New("every encode slot is taken")
	ErrNotAvailable = errors.New("ffmpeg is not installed")
)

type Encoder struct {
	slots chan struct{}
	stall time.Duration
}

func NewEncoder(jobs int, stall time.Duration) *Encoder {
	if jobs <= 0 {
		jobs = runtime.NumCPU()
	}
	if stall <= 0 {
		stall = defaultStall
	}

	return &Encoder{slots: make(chan struct{}, jobs), stall: stall}
}

func (e *Encoder) Enabled() bool {
	return Available()
}

func (e *Encoder) Stall() time.Duration {
	return e.stall
}

func (e *Encoder) Open(ctx context.Context, spec Spec) (io.ReadCloser, error) {
	if !Available() {
		return nil, ErrNotAvailable
	}

	select {
	case e.slots <- struct{}{}:
	default:
		return nil, ErrBusy
	}

	output, err := start(ctx, spec)
	if err != nil {
		<-e.slots

		return nil, err
	}

	return &slot{ReadCloser: output, slots: e.slots}, nil
}

type slot struct {
	io.ReadCloser
	slots chan struct{}
	freed bool
}

func (s *slot) Close() error {
	if !s.freed {
		s.freed = true
		<-s.slots
	}

	return s.ReadCloser.Close()
}
