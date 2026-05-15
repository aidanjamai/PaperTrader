package research

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
)

// Embedder embeds text into fixed-dimension float32 vectors.
type Embedder interface {
	EmbedQuery(ctx context.Context, text string) ([]float32, int, error)
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, int, error)
	Model() string
}

// VoyageEmbedder calls the Voyage AI embeddings API (voyage-finance-2).
type VoyageEmbedder struct {
	httpClient *http.Client
	apiKey     string
	model      string
}

func NewVoyageEmbedder(apiKey string) *VoyageEmbedder {
	return &VoyageEmbedder{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		apiKey:     apiKey,
		model:      "voyage-finance-2",
	}
}

func (v *VoyageEmbedder) Model() string { return "voyage-finance-2" }

func (v *VoyageEmbedder) EmbedQuery(ctx context.Context, text string) ([]float32, int, error) {
	vecs, tokens, err := v.callAPI(ctx, []string{text}, "query")
	if err != nil {
		return nil, 0, err
	}
	if len(vecs) == 0 {
		return nil, 0, fmt.Errorf("voyage embed: empty response")
	}
	return vecs[0], tokens, nil
}

func (v *VoyageEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, int, error) {
	if len(texts) == 0 {
		return nil, 0, nil
	}
	return v.callAPI(ctx, texts, "document")
}

func (v *VoyageEmbedder) callAPI(ctx context.Context, inputs []string, inputType string) ([][]float32, int, error) {
	payload, err := json.Marshal(map[string]any{
		"input":      inputs,
		"model":      v.model,
		"input_type": inputType,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("voyage embed: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.voyageai.com/v1/embeddings",
		bytes.NewReader(payload))
	if err != nil {
		return nil, 0, fmt.Errorf("voyage embed: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+v.apiKey)

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("voyage embed: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Read a bounded body for context; never log headers or the request body.
		limited, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, 0, fmt.Errorf("voyage embed: status %d: %s", resp.StatusCode, limited)
	}

	var result struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
			Index     int       `json:"index"`
		} `json:"data"`
		Usage struct {
			TotalTokens int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, 0, fmt.Errorf("voyage embed: decode response: %w", err)
	}

	vecs := make([][]float32, len(result.Data))
	for _, d := range result.Data {
		if d.Index < 0 || d.Index >= len(vecs) {
			return nil, 0, fmt.Errorf("voyage embed: index %d out of range for %d inputs", d.Index, len(inputs))
		}
		vecs[d.Index] = d.Embedding
	}
	return vecs, result.Usage.TotalTokens, nil
}

// CachedEmbedder wraps an Embedder with a Redis-backed embedding cache.
// If redis is nil, it delegates directly to the underlying embedder.
// Cache key: embed:v3:sha256(text). Only EmbedQuery is cached;
// batch calls bypass the cache to keep things simple.
type CachedEmbedder struct {
	inner Embedder
	redis *redis.Client
}

const embedCacheTTL = 7 * 24 * time.Hour

func NewCachedEmbedder(inner Embedder, redisClient *redis.Client) *CachedEmbedder {
	return &CachedEmbedder{inner: inner, redis: redisClient}
}

func (c *CachedEmbedder) Model() string { return c.inner.Model() }

func (c *CachedEmbedder) EmbedQuery(ctx context.Context, text string) ([]float32, int, error) {
	if c.redis == nil {
		return c.inner.EmbedQuery(ctx, text)
	}

	key := embedCacheKey(text)

	if cached, err := c.redis.Get(ctx, key).Bytes(); err == nil {
		var vec []float32
		if err := gob.NewDecoder(bytes.NewReader(cached)).Decode(&vec); err == nil {
			return vec, len(text) / 4, nil
		}
	}

	vec, tokens, err := c.inner.EmbedQuery(ctx, text)
	if err != nil {
		return nil, 0, err
	}

	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(vec); err == nil {
		// Best-effort write; ignore cache errors.
		_ = c.redis.Set(ctx, key, buf.Bytes(), embedCacheTTL).Err()
	}

	return vec, tokens, nil
}

func (c *CachedEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, int, error) {
	return c.inner.EmbedBatch(ctx, texts)
}

func embedCacheKey(text string) string {
	sum := sha256.Sum256([]byte(text))
	return fmt.Sprintf("embed:v3:%x", sum)
}
