package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

// StockCache interface defines methods for caching stock prices
type StockCache interface {
	GetStock(symbol, date string) (*StockData, error)
	SetStock(symbol, date string, data *StockData, ttl time.Duration) error
	InvalidateStock(symbol string) error
}

// RedisStockCache implements StockCache using Redis
type RedisStockCache struct {
	client     *redis.Client
	ctx        context.Context
	defaultTTL time.Duration
}

// NewRedisStockCache creates a new Redis-based stock cache
func NewRedisStockCache(client *redis.Client) *RedisStockCache {
	return &RedisStockCache{
		client:     client,
		ctx:        context.Background(),
		defaultTTL: 15 * time.Minute,
	}
}

// GetStock retrieves stock data from Redis cache
func (c *RedisStockCache) GetStock(symbol, date string) (*StockData, error) {
	key := fmt.Sprintf("stock:%s:%s", symbol, date)

	val, err := c.client.Get(c.ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			// Cache miss - return nil, nil (not an error)
			return nil, nil
		}
		// Redis error - log but don't fail, return nil to allow fallback
		log.Printf("[StockCache] Redis error getting stock %s:%s: %v", symbol, date, err)
		return nil, nil
	}

	var stockData StockData
	if err := json.Unmarshal([]byte(val), &stockData); err != nil {
		log.Printf("[StockCache] Error unmarshaling stock data for %s:%s: %v", symbol, date, err)
		return nil, nil
	}

	return &stockData, nil
}

// SetStock stores stock data in Redis cache with TTL
func (c *RedisStockCache) SetStock(symbol, date string, data *StockData, ttl time.Duration) error {
	if ttl == 0 {
		ttl = c.defaultTTL
	}

	key := fmt.Sprintf("stock:%s:%s", symbol, date)

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("error marshaling stock data: %w", err)
	}

	if err := c.client.Set(c.ctx, key, jsonData, ttl).Err(); err != nil {
		log.Printf("[StockCache] Error setting stock cache for %s:%s: %v", symbol, date, err)
		return err
	}

	return nil
}

// InvalidateStock removes all cache entries for a given symbol
func (c *RedisStockCache) InvalidateStock(symbol string) error {
	pattern := fmt.Sprintf("stock:%s:*", symbol)

	keys, err := c.client.Keys(c.ctx, pattern).Result()
	if err != nil {
		log.Printf("[StockCache] Error finding keys to invalidate for symbol %s: %v", symbol, err)
		return err
	}

	if len(keys) > 0 {
		if err := c.client.Del(c.ctx, keys...).Err(); err != nil {
			log.Printf("[StockCache] Error deleting cache keys for symbol %s: %v", symbol, err)
			return err
		}
		log.Printf("[StockCache] Invalidated %d cache entries for symbol %s", len(keys), symbol)
	}

	return nil
}

