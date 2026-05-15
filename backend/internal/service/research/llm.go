package research

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// LLMOpts configures a single LLM call.
type LLMOpts struct {
	Temperature float64
	MaxTokens   int
	JSONMode    bool
}

// LLMResult holds the raw output from an LLM call.
type LLMResult struct {
	Content          string
	PromptTokens     int
	CompletionTokens int
}

// LLMClient is the interface all generation backends must satisfy.
type LLMClient interface {
	Generate(ctx context.Context, system, user string, opts LLMOpts) (LLMResult, error)
	Model() string
	// PriceMicrosPer1KTokens returns the list-price in USD micros per 1 000 tokens
	// for input and output respectively. Free-tier callers still use these values
	// to record a "would-be cost" metric.
	PriceMicrosPer1KTokens() (in, out int)
}

// GroqClient implements LLMClient against the Groq OpenAI-compatible endpoint.
type GroqClient struct {
	httpClient *http.Client
	apiKey     string
	model      string
}

func NewGroqClient(apiKey string) *GroqClient {
	return &GroqClient{
		httpClient: &http.Client{Timeout: 60 * time.Second},
		apiKey:     apiKey,
		model:      "llama-3.3-70b-versatile",
	}
}

// NewGroqClientWithModel constructs a GroqClient targeting the given model.
// Useful when a caller needs a different model than the default generation model
// (e.g., a lightweight judge model for eval).
func NewGroqClientWithModel(apiKey, model string) *GroqClient {
	return &GroqClient{
		httpClient: &http.Client{Timeout: 60 * time.Second},
		apiKey:     apiKey,
		model:      model,
	}
}

func (g *GroqClient) Model() string { return g.model }

// groqModelPrices maps known Groq model names to their USD-micros-per-1K-token rates.
// Source: Groq published list prices (May 2026).
// Unknown models fall back to llama-3.3-70b-versatile rates rather than zero to
// avoid silently reporting $0 cost for new models added before this table is updated.
var groqModelPrices = map[string]struct{ in, out int }{
	"llama-3.3-70b-versatile": {59, 79},
	"llama-3.1-8b-instant":    {5, 8},
}

// PriceMicrosPer1KTokens returns Groq's published rates in USD micros per 1 000
// tokens for the client's configured model. Falls back to llama-3.3-70b-versatile
// rates for unrecognised models so cost is over-estimated rather than silently zero.
func (g *GroqClient) PriceMicrosPer1KTokens() (in, out int) {
	if p, ok := groqModelPrices[g.model]; ok {
		return p.in, p.out
	}
	return 59, 79
}

func (g *GroqClient) Generate(ctx context.Context, system, user string, opts LLMOpts) (LLMResult, error) {
	temp := opts.Temperature
	maxTok := opts.MaxTokens
	if maxTok == 0 {
		maxTok = 800
	}

	type message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	type responseFormat struct {
		Type string `json:"type"`
	}

	body := map[string]any{
		"model":       g.model,
		"messages":    []message{{Role: "system", Content: system}, {Role: "user", Content: user}},
		"temperature": temp,
		"max_tokens":  maxTok,
	}
	if opts.JSONMode {
		body["response_format"] = responseFormat{Type: "json_object"}
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return LLMResult{}, fmt.Errorf("groq generate: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.groq.com/openai/v1/chat/completions",
		bytes.NewReader(payload))
	if err != nil {
		return LLMResult{}, fmt.Errorf("groq generate: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+g.apiKey)

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return LLMResult{}, fmt.Errorf("groq generate: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		limited, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return LLMResult{}, fmt.Errorf("groq generate: status %d: %s", resp.StatusCode, limited)
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return LLMResult{}, fmt.Errorf("groq generate: decode response: %w", err)
	}
	if len(result.Choices) == 0 {
		return LLMResult{}, fmt.Errorf("groq generate: empty choices in response")
	}

	return LLMResult{
		Content:          result.Choices[0].Message.Content,
		PromptTokens:     result.Usage.PromptTokens,
		CompletionTokens: result.Usage.CompletionTokens,
	}, nil
}
