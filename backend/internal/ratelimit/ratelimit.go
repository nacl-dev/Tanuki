// Package ratelimit provides a simple token-bucket rate limiter for external API calls.
package ratelimit

import (
	"sync"
	"time"
)

// Limiter enforces a minimum interval between calls. It is safe for concurrent use.
type Limiter struct {
	mu       sync.Mutex
	interval time.Duration
	last     time.Time
}

// New creates a Limiter that allows at most one call per interval.
func New(interval time.Duration) *Limiter {
	return &Limiter{interval: interval}
}

// Wait blocks until the minimum interval has elapsed since the last call,
// then records the current time as the new last-call time.
func (l *Limiter) Wait() {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.interval <= 0 {
		return
	}
	now := time.Now()
	elapsed := now.Sub(l.last)
	if elapsed < l.interval {
		time.Sleep(l.interval - elapsed)
	}
	l.last = time.Now()
}
