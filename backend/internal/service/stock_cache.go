package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

// StockCache interface defines methods for caching stock prices
type StockCache interface {
	GetStock(ctx context.Context, symbol, date string) (*StockData, error)
	SetStock(ctx context.Context, symbol, date string, data *StockData, ttl time.Duration) error
	InvalidateStock(ctx context.Context, symbol string) error
}

// RedisStockCache implements StockCache using Redis
type RedisStockCache struct {
	client     *redis.Client
	defaultTTL time.Duration
}

// NewRedisStockCache creates a new Redis-based stock cache
func NewRedisStockCache(client *redis.Client) *RedisStockCache {
	return &RedisStockCache{
		client:     client,
		defaultTTL: 15 * time.Minute,
	}
}

// GetStock retrieves stock data from Redis cache
func (c *RedisStockCache) GetStock(ctx context.Context, symbol, date string) (*StockData, error) {
	key := fmt.Sprintf("stock:%s:%s", symbol, date)

	val, err := c.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			// Cache miss - return nil, nil (not an error)
			return nil, nil
		}
		// Redis error - log but don't fail, return nil to allow fallback
		slog.Error("Redis error getting stock from cache",
			"symbol", symbol,
			"date", date,
			"err", err,
			"component", "stock_cache",
		)
		return nil, nil
	}

	var stockData StockData
	if err := json.Unmarshal([]byte(val), &stockData); err != nil {
		slog.Error("failed to unmarshal stock cache entry",
			"symbol", symbol,
			"date", date,
			"err", err,
			"component", "stock_cache",
		)
		return nil, nil
	}

	return &stockData, nil
}

// SetStock stores stock data in Redis cache with TTL
func (c *RedisStockCache) SetStock(ctx context.Context, symbol, date string, data *StockData, ttl time.Duration) error {
	if ttl == 0 {
		ttl = c.defaultTTL
	}

	key := fmt.Sprintf("stock:%s:%s", symbol, date)

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("error marshaling stock data: %w", err)
	}

	if err := c.client.Set(ctx, key, jsonData, ttl).Err(); err != nil {
		slog.Error("failed to set stock cache entry",
			"symbol", symbol,
			"date", date,
			"err", err,
			"component", "stock_cache",
		)
		return err
	}

	return nil
}

// InvalidateStock removes all cache entries for a given symbol
func (c *RedisStockCache) InvalidateStock(ctx context.Context, symbol string) error {
	pattern := fmt.Sprintf("stock:%s:*", symbol)

	keys, err := c.client.Keys(ctx, pattern).Result()
	if err != nil {
		slog.Error("failed to find cache keys for invalidation",
			"symbol", symbol,
			"err", err,
			"component", "stock_cache",
		)
		return err
	}

	if len(keys) > 0 {
		if err := c.client.Del(ctx, keys...).Err(); err != nil {
			slog.Error("failed to delete cache keys for symbol",
				"symbol", symbol,
				"err", err,
				"component", "stock_cache",
			)
			return err
		}
		slog.Info("invalidated stock cache entries", "symbol", symbol, "count", len(keys), "component", "stock_cache")
	}

	return nil
}
