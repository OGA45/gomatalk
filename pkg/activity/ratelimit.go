package activity

import (
	"sync"
	"time"
)

// rateLimiter is an in-memory token-bucket keyed by an arbitrary string
// (client IP for /api/token, user ID for mutating calls). Buckets refill
// continuously and are pruned lazily so long-idle keys do not accumulate.
type rateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*tokenBucket
	calls   int
}

type tokenBucket struct {
	tokens float64
	last   time.Time
}

func newRateLimiter() *rateLimiter {
	return &rateLimiter{buckets: make(map[string]*tokenBucket)}
}

// allow reports whether one request is permitted for key. capacity is the
// burst size and ratePerMin the sustained refill rate (tokens per minute).
func (rl *rateLimiter) allow(key string, ratePerMin, capacity float64) bool {
	now := time.Now()
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.calls++
	if rl.calls%256 == 0 {
		rl.prune(now)
	}

	b := rl.buckets[key]
	if b == nil {
		b = &tokenBucket{tokens: capacity, last: now}
		rl.buckets[key] = b
	} else {
		elapsed := now.Sub(b.last).Minutes()
		b.tokens += elapsed * ratePerMin
		if b.tokens > capacity {
			b.tokens = capacity
		}
		b.last = now
	}

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// prune drops buckets untouched for over 10 minutes — they would have fully
// refilled by then, so forgetting them is equivalent to keeping them.
func (rl *rateLimiter) prune(now time.Time) {
	for k, b := range rl.buckets {
		if now.Sub(b.last) > 10*time.Minute {
			delete(rl.buckets, k)
		}
	}
}
