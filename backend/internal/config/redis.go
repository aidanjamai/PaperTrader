package config

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

func ConnectRedis(cfg *Config) (*redis.Client, error) {
	var client *redis.Client
	var err error

	// Parse Redis URL or use individual components
	opts := &redis.Options{
		DB: cfg.RedisDB,
	}

	if cfg.RedisURL != "" {
		opt, err := redis.ParseURL(cfg.RedisURL)
		if err != nil {
			return nil, fmt.Errorf("failed to parse Redis URL: %w", err)
		}
		opts.Addr = opt.Addr
		opts.Password = opt.Password
		opts.DB = cfg.RedisDB
	} else {
		opts.Addr = "localhost:6379"
	}

	if cfg.RedisPassword != "" {
		opts.Password = cfg.RedisPassword
	}

	// Retry loop similar to MongoDB connection. Each attempt gets its own
	// context with timeout that's cancelled before the next iteration —
	// previously cancel() was deferred, so all 5 cancels piled up to the
	// function exit instead of releasing per-attempt.
	for i := 0; i < 5; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

		slog.Info("attempting to connect to Redis", "addr", opts.Addr, "attempt", i+1, "max_attempts", 5)

		client = redis.NewClient(opts)

		// Test the connection
		err = client.Ping(ctx).Err()
		cancel()
		if err != nil {
			slog.Warn("failed to ping Redis", "attempt", i+1, "max_attempts", 5, "err", err)
			client.Close()
			if i < 4 {
				time.Sleep(5 * time.Second)
			}
			continue
		}

		slog.Info("connected to Redis successfully", "addr", opts.Addr)
		return client, nil
	}

	return nil, fmt.Errorf("failed to connect to Redis after 5 attempts: %w", err)
}
