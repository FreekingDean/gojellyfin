package tmdb

import (
	"context"
	"sync"
	"time"
)

type limiter struct {
	mutex   sync.Mutex
	spacing time.Duration
	next    time.Time
}

func newLimiter(spacing time.Duration) *limiter {
	return &limiter{spacing: spacing}
}

func (l *limiter) wait(ctx context.Context) error {
	l.mutex.Lock()
	now := time.Now()
	if l.next.Before(now) {
		l.next = now
	}
	delay := l.next.Sub(now)
	l.next = l.next.Add(l.spacing)
	l.mutex.Unlock()

	if delay <= 0 {
		return nil
	}

	return sleep(ctx, delay)
}
