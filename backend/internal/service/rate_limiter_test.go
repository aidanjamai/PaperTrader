package service

import (
	"context"
	"testing"
	"time"
)

func TestMemoryRateLimiter_AllowsUnderLimit(t *testing.T) {
	rl := NewMemoryRateLimiter()
	for i := 0; i < 50; i++ {
		result, err := rl.CheckLimit(context.Background(), "", "127.0.0.1")
		if err != nil {
			t.Fatalf("CheckLimit: %v", err)
		}
		if !result.Allowed {
			t.Fatalf("request %d should be allowed (under IP limit)", i+1)
		}
	}
}

func TestMemoryRateLimiter_BlocksAtIPLimit(t *testing.T) {
	rl := &MemoryRateLimiter{
		counts:    make(map[string][]time.Time),
		userLimit: DefaultUserLimit,
		ipLimit:   3,
		window:    DefaultWindowDuration,
	}

	for i := 0; i < 3; i++ {
		r, _ := rl.CheckLimit(context.Background(), "", "10.0.0.1")
		if !r.Allowed {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}

	r, _ := rl.CheckLimit(context.Background(), "", "10.0.0.1")
	if r.Allowed {
		t.Error("4th request should be blocked by IP limit")
	}
	if !r.LimitExceeded {
		t.Error("LimitExceeded should be true")
	}
}

func TestMemoryRateLimiter_BlocksAtUserLimit(t *testing.T) {
	rl := &MemoryRateLimiter{
		counts:    make(map[string][]time.Time),
		userLimit: 2,
		ipLimit:   DefaultIPLimit,
		window:    DefaultWindowDuration,
	}

	rl.CheckLimit(context.Background(), "user-1", "10.0.0.1")
	rl.CheckLimit(context.Background(), "user-1", "10.0.0.2") // different IP, same user

	r, _ := rl.CheckLimit(context.Background(), "user-1", "10.0.0.3")
	if r.Allowed {
		t.Error("3rd request for same user should be blocked")
	}
}

func TestMemoryRateLimiter_IndependentPerIP(t *testing.T) {
	rl := &MemoryRateLimiter{
		counts:    make(map[string][]time.Time),
		userLimit: DefaultUserLimit,
		ipLimit:   1,
		window:    DefaultWindowDuration,
	}

	r1, _ := rl.CheckLimit(context.Background(), "", "10.0.0.1")
	r2, _ := rl.CheckLimit(context.Background(), "", "10.0.0.2")
	if !r1.Allowed || !r2.Allowed {
		t.Error("different IPs should be rate limited independently")
	}

	r3, _ := rl.CheckLimit(context.Background(), "", "10.0.0.1")
	if r3.Allowed {
		t.Error("second request from same IP should be blocked when limit is 1")
	}
}

func TestMemoryRateLimiter_WindowExpiry(t *testing.T) {
	rl := &MemoryRateLimiter{
		counts:    make(map[string][]time.Time),
		userLimit: DefaultUserLimit,
		ipLimit:   2,
		window:    80 * time.Millisecond,
	}

	rl.CheckLimit(context.Background(), "", "10.0.0.1")
	rl.CheckLimit(context.Background(), "", "10.0.0.1")

	r, _ := rl.CheckLimit(context.Background(), "", "10.0.0.1")
	if r.Allowed {
		t.Fatal("3rd request in window should be blocked")
	}

	time.Sleep(90 * time.Millisecond)

	r, _ = rl.CheckLimit(context.Background(), "", "10.0.0.1")
	if !r.Allowed {
		t.Error("request after window expiry should be allowed")
	}
}

func TestMemoryRateLimiter_RemainingDecreases(t *testing.T) {
	rl := &MemoryRateLimiter{
		counts:    make(map[string][]time.Time),
		userLimit: DefaultUserLimit,
		ipLimit:   5,
		window:    DefaultWindowDuration,
	}

	var prev int = 5
	for i := 0; i < 4; i++ {
		r, _ := rl.CheckLimit(context.Background(), "", "10.0.0.1")
		if r.Remaining >= prev {
			t.Errorf("remaining should decrease: got %d, previous %d", r.Remaining, prev)
		}
		prev = r.Remaining
	}
}
