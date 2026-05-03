package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

// HistoricalCache interface defines methods for caching historical stock data
type HistoricalCache interface {
	GetHistorical(ctx context.Context, symbol, startDate, endDate string) (*HistoricalData, error)
	SetHistorical(ctx context.Context, symbol, startDate, endDate string, data *HistoricalData, ttl time.Duration) error
}

// RedisHistoricalCache implements HistoricalCache using Redis
type RedisHistoricalCache struct {
	client     *redis.Client
	defaultTTL time.Duration
}

// NewRedisHistoricalCache creates a new Redis-based historical data cache
func NewRedisHistoricalCache(client *redis.Client) *RedisHistoricalCache {
	return &RedisHistoricalCache{
		client:     client,
		defaultTTL: 24 * time.Hour, // Daily cache
	}
}

// GetHistorical retrieves historical data from Redis cache
func (c *RedisHistoricalCache) GetHistorical(ctx context.Context, symbol, startDate, endDate string) (*HistoricalData, error) {
	key := fmt.Sprintf("historical:%s:%s:%s", symbol, startDate, endDate)

	val, err := c.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			// Cache miss - return nil, nil (not an error)
			return nil, nil
		}
		// Redis error - log but don't fail, return nil to allow fallback
		slog.Error("Redis error getting historical data",
			"symbol", symbol,
			"start_date", startDate,
			"end_date", endDate,
			"err", err,
			"component", "historical_cache",
		)
		return nil, nil
	}

	var historicalData HistoricalData
	if err := json.Unmarshal([]byte(val), &historicalData); err != nil {
		slog.Error("failed to unmarshal historical cache entry",
			"symbol", symbol,
			"start_date", startDate,
			"end_date", endDate,
			"err", err,
			"component", "historical_cache",
		)
		return nil, nil
	}

	return &historicalData, nil
}

// SetHistorical stores historical data in Redis cache with TTL
func (c *RedisHistoricalCache) SetHistorical(ctx context.Context, symbol, startDate, endDate string, data *HistoricalData, ttl time.Duration) error {
	if ttl == 0 {
		ttl = c.defaultTTL
	}

	key := fmt.Sprintf("historical:%s:%s:%s", symbol, startDate, endDate)

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("error marshaling historical data: %w", err)
	}

	if err := c.client.Set(ctx, key, jsonData, ttl).Err(); err != nil {
		slog.Error("failed to set historical cache entry",
			"symbol", symbol,
			"start_date", startDate,
			"end_date", endDate,
			"err", err,
			"component", "historical_cache",
		)
		return err
	}

	return nil
}
