package server

import (
	"sync"
	"time"
)

// limiter is a token bucket shared by every client: the page has a single
// reader, so a global budget is enough to keep the Claude login safe and
// can't be dodged by spoofing client addresses.
type limiter struct {
	burst, perMinute float64

	mu     sync.Mutex
	tokens float64
	last   time.Time
}

func newLimiter(burst, perMinute float64) *limiter {
	return &limiter{burst: burst, perMinute: perMinute}
}

// allow takes a token if one is left.
func (l *limiter) allow(now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.last.IsZero() {
		l.tokens = l.burst
	} else {
		l.tokens = min(l.burst, l.tokens+now.Sub(l.last).Minutes()*l.perMinute)
	}
	l.last = now
	if l.tokens < 1 {
		return false
	}
	l.tokens--
	return true
}
