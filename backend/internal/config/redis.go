package config

import (
	"context"
	"fmt"
	"log"
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

	// Retry loop similar to MongoDB connection
	for i := 0; i < 5; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		log.Printf("Attempting to connect to Redis at %s (attempt %d/5)", opts.Addr, i+1)

		client = redis.NewClient(opts)

		// Test the connection
		err = client.Ping(ctx).Err()
		if err != nil {
			log.Printf("Failed to ping Redis (attempt %d/5): %v", i+1, err)
			client.Close()
			if i < 4 {
				time.Sleep(5 * time.Second)
			}
			continue
		}

		log.Println("Connected to Redis successfully")
		return client, nil
	}

	return nil, fmt.Errorf("failed to connect to Redis after 5 attempts: %w", err)
}

