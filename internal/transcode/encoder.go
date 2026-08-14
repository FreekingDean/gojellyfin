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

// An Encoder runs ffmpeg in this process. There is no second hop: the pod that
// serves the stream is the pod that encodes it, and which pods those are is a
// routing decision rather than a protocol.
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

// An encode saturates about a core, so refusing past that is what keeps a
// fourth stream from making the other three slower without finishing sooner.
// The refusal is a 503 the gateway can retry against another pod, which is the
// whole of the load balancing.
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

// The slot is held until the client is done with the stream, not until ffmpeg
// is started, because it is the running encode that costs the core.
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
