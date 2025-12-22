package service

import (
	"context"
	"fmt"
	"log"
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
	CheckLimit(userID, ipAddress string) (*RateLimitResult, error)
}

// RedisRateLimiter implements RateLimiter using Redis sliding window
type RedisRateLimiter struct {
	client         *redis.Client
	ctx            context.Context
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
		ctx:            context.Background(),
		userLimit:      DefaultUserLimit,
		ipLimit:        DefaultIPLimit,
		windowDuration: DefaultWindowDuration,
	}
}

// CheckLimit checks both user and IP rate limits using sliding window algorithm
func (r *RedisRateLimiter) CheckLimit(userID, ipAddress string) (*RateLimitResult, error) {
	now := time.Now()
	windowStart := now.Add(-r.windowDuration)

	result := &RateLimitResult{
		ResetTime: now.Add(r.windowDuration),
	}

	// Check user limit if userID is provided
	if userID != "" {
		userAllowed, userRemaining, err := r.checkWindowLimit("ratelimit:user:"+userID, r.userLimit, windowStart)
		if err != nil {
			log.Printf("[RateLimiter] Error checking user limit for %s: %v", userID, err)
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
	ipAllowed, ipRemaining, err := r.checkWindowLimit("ratelimit:ip:"+ipAddress, r.ipLimit, windowStart)
	if err != nil {
		log.Printf("[RateLimiter] Error checking IP limit for %s: %v", ipAddress, err)
		// Fail open - allow request if Redis is unavailable
		if result.Remaining == 0 {
			result.Remaining = r.ipLimit
		}
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

// checkWindowLimit implements sliding window rate limiting using sorted sets
func (r *RedisRateLimiter) checkWindowLimit(key string, limit int, windowStart time.Time) (allowed bool, remaining int, err error) {
	now := time.Now()
	member := fmt.Sprintf("%d", now.UnixNano())
	score := float64(now.UnixNano())

	pipe := r.client.Pipeline()

	// Remove old entries outside the window
	pipe.ZRemRangeByScore(r.ctx, key, "0", fmt.Sprintf("%d", windowStart.UnixNano()))

	// Count current entries in window
	countCmd := pipe.ZCard(r.ctx, key)

	// Add current request
	pipe.ZAdd(r.ctx, key, redis.Z{
		Score:  score,
		Member: member,
	})

	// Set expiration on the key
	pipe.Expire(r.ctx, key, r.windowDuration+time.Minute)

	_, err = pipe.Exec(r.ctx)
	if err != nil {
		return false, 0, err
	}

	currentCount := int(countCmd.Val())
	remaining = limit - currentCount - 1 // -1 because we just added the current request

	if currentCount >= limit {
		return false, 0, nil
	}

	return true, remaining, nil
}

