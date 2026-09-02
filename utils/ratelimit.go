package utils

import (
	"sync"
	"time"
)

// DiscordRateLimiter provides simple rate limiting for Discord API calls.
// Discord's guild update endpoint: ~10 requests per 10 seconds.
type DiscordRateLimiter struct {
	mu       sync.Mutex
	lastCall time.Time
	minGap   time.Duration
}

// NewDiscordRateLimiter creates a rate limiter for Discord API calls.
// minGap is the minimum time between calls (e.g., 1 second for 10 req/10s).
func NewDiscordRateLimiter(minGap time.Duration) *DiscordRateLimiter {
	return &DiscordRateLimiter{
		minGap: minGap,
	}
}

// Wait blocks until it's safe to make the next API call.
func (r *DiscordRateLimiter) Wait() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.lastCall.IsZero() {
		elapsed := time.Since(r.lastCall)
		if elapsed < r.minGap {
			time.Sleep(r.minGap - elapsed)
		}
	}
	r.lastCall = time.Now()
}
