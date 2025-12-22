package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

// HistoricalCache interface defines methods for caching historical stock data
type HistoricalCache interface {
	GetHistorical(symbol, startDate, endDate string) (*HistoricalData, error)
	SetHistorical(symbol, startDate, endDate string, data *HistoricalData, ttl time.Duration) error
}

// RedisHistoricalCache implements HistoricalCache using Redis
type RedisHistoricalCache struct {
	client     *redis.Client
	ctx        context.Context
	defaultTTL time.Duration
}

// NewRedisHistoricalCache creates a new Redis-based historical data cache
func NewRedisHistoricalCache(client *redis.Client) *RedisHistoricalCache {
	return &RedisHistoricalCache{
		client:     client,
		ctx:        context.Background(),
		defaultTTL: 24 * time.Hour, // Daily cache
	}
}

// GetHistorical retrieves historical data from Redis cache
func (c *RedisHistoricalCache) GetHistorical(symbol, startDate, endDate string) (*HistoricalData, error) {
	key := fmt.Sprintf("historical:%s:%s:%s", symbol, startDate, endDate)

	val, err := c.client.Get(c.ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			// Cache miss - return nil, nil (not an error)
			return nil, nil
		}
		// Redis error - log but don't fail, return nil to allow fallback
		log.Printf("[HistoricalCache] Redis error getting historical data %s:%s:%s: %v", symbol, startDate, endDate, err)
		return nil, nil
	}

	var historicalData HistoricalData
	if err := json.Unmarshal([]byte(val), &historicalData); err != nil {
		log.Printf("[HistoricalCache] Error unmarshaling historical data for %s:%s:%s: %v", symbol, startDate, endDate, err)
		return nil, nil
	}

	return &historicalData, nil
}

// SetHistorical stores historical data in Redis cache with TTL
func (c *RedisHistoricalCache) SetHistorical(symbol, startDate, endDate string, data *HistoricalData, ttl time.Duration) error {
	if ttl == 0 {
		ttl = c.defaultTTL
	}

	key := fmt.Sprintf("historical:%s:%s:%s", symbol, startDate, endDate)

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("error marshaling historical data: %w", err)
	}

	if err := c.client.Set(c.ctx, key, jsonData, ttl).Err(); err != nil {
		log.Printf("[HistoricalCache] Error setting historical cache for %s:%s:%s: %v", symbol, startDate, endDate, err)
		return err
	}

	return nil
}

