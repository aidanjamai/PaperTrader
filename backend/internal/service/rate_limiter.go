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
	// CheckLimitWithBucket runs the same sliding-window check against a custom
	// bucket namespace and limit/window pair. Lets a single endpoint enforce
	// tighter limits than the global default without colliding with the
	// global ratelimit:user / ratelimit:ip keys.
	CheckLimitWithBucket(ctx context.Context, bucket, userID, ipAddress string, userLimit, ipLimit int, window time.Duration) (*RateLimitResult, error)
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

// CheckLimit checks both user and IP rate limits against the global default
// bucket using the limiter's configured limits and window.
func (r *RedisRateLimiter) CheckLimit(ctx context.Context, userID, ipAddress string) (*RateLimitResult, error) {
	return r.CheckLimitWithBucket(ctx, "ratelimit", userID, ipAddress, r.userLimit, r.ipLimit, r.windowDuration)
}

// CheckLimitWithBucket runs the sliding-window check against a custom bucket
// namespace and per-call limits/window. Used by endpoints that need tighter
// limits than the global default (e.g. /api/research/ask).
func (r *RedisRateLimiter) CheckLimitWithBucket(ctx context.Context, bucket, userID, ipAddress string, userLimit, ipLimit int, window time.Duration) (*RateLimitResult, error) {
	now := time.Now()
	windowStart := now.Add(-window)

	result := &RateLimitResult{
		ResetTime: now.Add(window),
	}

	if userID != "" {
		userAllowed, userRemaining, err := r.checkWindowLimitWithTTL(ctx, bucket+":user:"+userID, userLimit, windowStart, now, window)
		if err != nil {
			slog.Warn("Redis error checking user rate limit",
				"bucket", bucket,
				"user_id", userID,
				"err", err,
				"component", "rate_limiter",
			)
			return &RateLimitResult{Allowed: true, Remaining: userLimit}, nil
		}
		if !userAllowed {
			result.Allowed = false
			result.LimitExceeded = true
			result.Remaining = 0
			return result, nil
		}
		result.Remaining = userRemaining
	}

	ipAllowed, ipRemaining, err := r.checkWindowLimitWithTTL(ctx, bucket+":ip:"+ipAddress, ipLimit, windowStart, now, window)
	if err != nil {
		slog.Warn("Redis error checking IP rate limit",
			"bucket", bucket,
			"remote_addr", ipAddress,
			"err", err,
			"component", "rate_limiter",
		)
		fallback := ipLimit
		if userLimit < fallback {
			fallback = userLimit
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

	result.Allowed = true
	if userID != "" && result.Remaining > ipRemaining {
		result.Remaining = ipRemaining
	} else if userID == "" {
		result.Remaining = ipRemaining
	}

	return result, nil
}

// checkWindowLimitWithTTL implements sliding window rate limiting using sorted
// sets, with the TTL derived from the caller-supplied window. The check-and-add
// must be atomic; see slidingWindowScript above.
func (r *RedisRateLimiter) checkWindowLimitWithTTL(ctx context.Context, key string, limit int, windowStart, now time.Time, window time.Duration) (allowed bool, remaining int, err error) {
	ttl := int((window + time.Minute) / time.Second)

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
