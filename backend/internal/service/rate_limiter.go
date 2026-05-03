package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

// RateLimitResult contains information about rate limit check
type RateLimitResult struct {
	Allowed       bool
	Remaining     int
	ResetTime     time.Time
	LimitExceeded bool
}

// RateLimiter interface defines methods for rate limiting
type RateLimiter interface {
	CheckLimit(ctx context.Context, userID, ipAddress string) (*RateLimitResult, error)
}

// RedisRateLimiter implements RateLimiter using Redis sliding window
type RedisRateLimiter struct {
	client         *redis.Client
	userLimit      int           // requests per window
	ipLimit        int           // requests per window
	windowDuration time.Duration // time window
}

const (
	// DefaultUserLimit is the default rate limit for authenticated users (100 requests/hour)
	DefaultUserLimit = 100
	// DefaultIPLimit is the default rate limit for IP addresses (200 requests/hour)
	DefaultIPLimit = 200
	// DefaultWindowDuration is the default time window (1 hour)
	DefaultWindowDuration = time.Hour
)

// NewRedisRateLimiter creates a new Redis-based rate limiter
func NewRedisRateLimiter(client *redis.Client) *RedisRateLimiter {
	return &RedisRateLimiter{
		client:         client,
		userLimit:      DefaultUserLimit,
		ipLimit:        DefaultIPLimit,
		windowDuration: DefaultWindowDuration,
	}
}

// slidingWindowScript is an atomic check-and-add for the sliding window.
// Without this in a single EVAL, the check and the ZADD race: two requests can
// both see count=limit-1, both pass, and both end up in the set, exceeding the
// limit. Equally, a non-atomic pipeline that ZADDs unconditionally lets every
// rejected request consume a slot, decaying the effective limit over time.
//
// KEYS[1] = sorted-set key
// ARGV[1] = window-start score (nanoseconds)
// ARGV[2] = now score (nanoseconds; also used as the unique member)
// ARGV[3] = limit
// ARGV[4] = key TTL seconds
// Returns: {allowed (1|0), count_after}
var slidingWindowScript = redis.NewScript(`
local key       = KEYS[1]
local windowMin = tonumber(ARGV[1])
local now       = tonumber(ARGV[2])
local limit     = tonumber(ARGV[3])
local ttl       = tonumber(ARGV[4])

redis.call('ZREMRANGEBYSCORE', key, 0, windowMin)
local count = redis.call('ZCARD', key)
if count >= limit then
  return {0, count}
end
redis.call('ZADD', key, now, tostring(now))
redis.call('EXPIRE', key, ttl)
return {1, count + 1}
`)

// CheckLimit checks both user and IP rate limits using sliding window algorithm
func (r *RedisRateLimiter) CheckLimit(ctx context.Context, userID, ipAddress string) (*RateLimitResult, error) {
	now := time.Now()
	windowStart := now.Add(-r.windowDuration)

	result := &RateLimitResult{
		ResetTime: now.Add(r.windowDuration),
	}

	// Check user limit if userID is provided
	if userID != "" {
		userAllowed, userRemaining, err := r.checkWindowLimit(ctx, "ratelimit:user:"+userID, r.userLimit, windowStart, now)
		if err != nil {
			slog.Warn("Redis error checking user rate limit",
				"user_id", userID,
				"err", err,
				"component", "rate_limiter",
			)
			// Fail open - allow request if Redis is unavailable
			return &RateLimitResult{Allowed: true, Remaining: r.userLimit}, nil
		}
		if !userAllowed {
			result.Allowed = false
			result.LimitExceeded = true
			result.Remaining = 0
			return result, nil
		}
		result.Remaining = userRemaining
	}

	// Check IP limit
	ipAllowed, ipRemaining, err := r.checkWindowLimit(ctx, "ratelimit:ip:"+ipAddress, r.ipLimit, windowStart, now)
	if err != nil {
		slog.Warn("Redis error checking IP rate limit",
			"remote_addr", ipAddress,
			"err", err,
			"component", "rate_limiter",
		)
		// Fail open: allow but report a generic remaining count.
		// Use the smaller of the two configured limits as the upper bound — it's
		// a placeholder header value, not a binding decision.
		fallback := r.ipLimit
		if r.userLimit < fallback {
			fallback = r.userLimit
		}
		result.Remaining = fallback
		result.Allowed = true
		return result, nil
	}
	if !ipAllowed {
		result.Allowed = false
		result.LimitExceeded = true
		result.Remaining = 0
		return result, nil
	}

	// Both limits passed
	result.Allowed = true
	// Return the more restrictive remaining count
	if userID != "" && result.Remaining > ipRemaining {
		result.Remaining = ipRemaining
	} else if userID == "" {
		result.Remaining = ipRemaining
	}

	return result, nil
}

// checkWindowLimit implements sliding window rate limiting using sorted sets.
// The check-and-add must be atomic; see slidingWindowScript above.
func (r *RedisRateLimiter) checkWindowLimit(ctx context.Context, key string, limit int, windowStart, now time.Time) (allowed bool, remaining int, err error) {
	ttl := int((r.windowDuration + time.Minute) / time.Second)

	res, err := slidingWindowScript.Run(ctx, r.client,
		[]string{key},
		windowStart.UnixNano(),
		now.UnixNano(),
		limit,
		ttl,
	).Result()
	if err != nil {
		return false, 0, err
	}

	arr, ok := res.([]interface{})
	if !ok || len(arr) != 2 {
		return false, 0, fmt.Errorf("unexpected rate limiter script result: %T", res)
	}
	allowedRaw, _ := arr[0].(int64)
	countRaw, _ := arr[1].(int64)

	if allowedRaw == 0 {
		return false, 0, nil
	}
	remaining = limit - int(countRaw)
	if remaining < 0 {
		remaining = 0
	}
	return true, remaining, nil
}
