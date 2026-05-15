package research

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/gob"
	"fmt"
	"log/slog"

	"github.com/redis/go-redis/v9"
)

// CachedLLMClient wraps any LLMClient with a Redis-backed persistent cache.
// TTL is 0 (no expiry) — appropriate for eval runs where replay stability matters
// more than cache freshness. Flush manually with redis-cli DEL or FLUSHDB.
type CachedLLMClient struct {
	inner       LLMClient
	redisClient *redis.Client
	keyPrefix   string
}

func NewCachedLLMClient(inner LLMClient, redisClient *redis.Client, keyPrefix string) *CachedLLMClient {
	return &CachedLLMClient{
		inner:       inner,
		redisClient: redisClient,
		keyPrefix:   keyPrefix,
	}
}

func (c *CachedLLMClient) Model() string { return c.inner.Model() }

func (c *CachedLLMClient) PriceMicrosPer1KTokens() (in, out int) {
	return c.inner.PriceMicrosPer1KTokens()
}

func (c *CachedLLMClient) Generate(ctx context.Context, system, user string, opts LLMOpts) (LLMResult, error) {
	if c.redisClient == nil {
		slog.Debug("llm cache bypassed: no redis client")
		return c.inner.Generate(ctx, system, user, opts)
	}

	key := c.cacheKey(system, user, opts)

	if cached, err := c.redisClient.Get(ctx, key).Bytes(); err == nil {
		var result LLMResult
		if gobErr := gob.NewDecoder(bytes.NewReader(cached)).Decode(&result); gobErr == nil {
			return result, nil
		}
	}

	result, err := c.inner.Generate(ctx, system, user, opts)
	if err != nil {
		return LLMResult{}, err
	}

	var buf bytes.Buffer
	if gobErr := gob.NewEncoder(&buf).Encode(result); gobErr == nil {
		// 0 TTL means no expiry — best-effort, ignore cache write errors.
		_ = c.redisClient.Set(ctx, key, buf.Bytes(), 0).Err()
	}

	return result, nil
}

func (c *CachedLLMClient) cacheKey(system, user string, opts LLMOpts) string {
	// Include all fields that can change the output so distinct calls never collide.
	// MaxTokens sits between Temperature and JSONMode; prefix collisions are harmless
	// here because eval:gen and eval:judge use different keyPrefix values, but
	// omitting MaxTokens would silently collide if two calls differed only there.
	raw := fmt.Sprintf("%s\x00%s\x00%s\x00%v\x00%d\x00%v",
		c.inner.Model(), system, user, opts.Temperature, opts.MaxTokens, opts.JSONMode)
	sum := sha256.Sum256([]byte(raw))
	return c.keyPrefix + fmt.Sprintf("%x", sum)
}
