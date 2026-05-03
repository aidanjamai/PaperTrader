package service

import (
	"context"
	"sync"
	"time"
)

// MemoryRateLimiter is an in-process sliding-window rate limiter used when Redis
// is unavailable. State is lost on restart; use RedisRateLimiter in production.
type MemoryRateLimiter struct {
	mu        sync.Mutex
	counts    map[string][]time.Time
	userLimit int
	ipLimit   int
	window    time.Duration
}

func NewMemoryRateLimiter() *MemoryRateLimiter {
	return &MemoryRateLimiter{
		counts:    make(map[string][]time.Time),
		userLimit: DefaultUserLimit,
		ipLimit:   DefaultIPLimit,
		window:    DefaultWindowDuration,
	}
}

// CheckLimit implements RateLimiter. It checks both user and IP sliding windows
// and increments the counter for the current request.
func (m *MemoryRateLimiter) CheckLimit(_ context.Context, userID, ipAddress string) (*RateLimitResult, error) {
	now := time.Now()
	cutoff := now.Add(-m.window)
	result := &RateLimitResult{ResetTime: now.Add(m.window)}

	m.mu.Lock()
	defer m.mu.Unlock()

	if userID != "" {
		allowed, remaining := m.checkAndAdd("user:"+userID, m.userLimit, cutoff, now)
		if !allowed {
			result.Allowed = false
			result.LimitExceeded = true
			return result, nil
		}
		result.Remaining = remaining
	}

	allowed, remaining := m.checkAndAdd("ip:"+ipAddress, m.ipLimit, cutoff, now)
	if !allowed {
		result.Allowed = false
		result.LimitExceeded = true
		return result, nil
	}

	result.Allowed = true
	if userID == "" || remaining < result.Remaining {
		result.Remaining = remaining
	}
	return result, nil
}

func (m *MemoryRateLimiter) checkAndAdd(key string, limit int, cutoff, now time.Time) (bool, int) {
	times := m.counts[key]

	// Prune entries outside the sliding window
	start := 0
	for start < len(times) && times[start].Before(cutoff) {
		start++
	}
	times = times[start:]

	if len(times) >= limit {
		m.counts[key] = times
		return false, 0
	}

	times = append(times, now)
	m.counts[key] = times
	return true, limit - len(times)
}
