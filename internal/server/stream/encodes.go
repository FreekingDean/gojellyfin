package stream

import (
	"sync"
	"time"
)

const supersedeWait = 5 * time.Second

type encode struct {
	stop func()
	done chan struct{}
	over bool
}

type encodes struct {
	mu      sync.Mutex
	running map[string]*encode
}

func (e *encodes) begin(token string, stop func()) func() {
	current := &encode{stop: stop, done: make(chan struct{})}

	e.mu.Lock()
	if e.running == nil {
		e.running = map[string]*encode{}
	}
	earlier := e.running[token]
	e.running[token] = current
	e.mu.Unlock()

	e.end(earlier)

	return func() {
		e.mu.Lock()
		current.over = true
		if e.running[token] == current {
			delete(e.running, token)
		}
		e.mu.Unlock()

		close(current.done)
	}
}

func (e *encodes) stop(token string) {
	e.mu.Lock()
	earlier := e.running[token]
	e.mu.Unlock()

	e.end(earlier)
}

func (e *encodes) end(earlier *encode) {
	if earlier == nil {
		return
	}

	e.mu.Lock()
	if !earlier.over {
		earlier.stop()
	}
	e.mu.Unlock()

	select {
	case <-earlier.done:
	case <-time.After(supersedeWait):
	}
}
