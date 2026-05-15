package research

import (
	"bytes"
	"context"
	"encoding/gob"
	"testing"
)

// stubCountLLM records how many times Generate is called.
// modelName defaults to "stub-model" when empty.
type stubCountLLM struct {
	result    LLMResult
	err       error
	calls     int
	modelName string
}

func (s *stubCountLLM) Generate(_ context.Context, _, _ string, _ LLMOpts) (LLMResult, error) {
	s.calls++
	return s.result, s.err
}
func (s *stubCountLLM) Model() string {
	if s.modelName != "" {
		return s.modelName
	}
	return "stub-model"
}
func (s *stubCountLLM) PriceMicrosPer1KTokens() (int, int) { return 10, 20 }

func TestCachedLLMClient_NilRedis_AlwaysDelegates(t *testing.T) {
	inner := &stubCountLLM{result: LLMResult{Content: "hello", PromptTokens: 5, CompletionTokens: 3}}
	c := NewCachedLLMClient(inner, nil, "test:")

	for i := 0; i < 3; i++ {
		res, err := c.Generate(context.Background(), "sys", "usr", LLMOpts{})
		if err != nil {
			t.Fatalf("call %d: unexpected error: %v", i+1, err)
		}
		if res.Content != "hello" {
			t.Errorf("call %d: content = %q, want %q", i+1, res.Content, "hello")
		}
	}
	if inner.calls != 3 {
		t.Errorf("inner.calls = %d, want 3", inner.calls)
	}
}

func TestCachedLLMClient_NilRedis_TokensPreserved(t *testing.T) {
	inner := &stubCountLLM{result: LLMResult{Content: "ok", PromptTokens: 42, CompletionTokens: 7}}
	c := NewCachedLLMClient(inner, nil, "test:")

	res, err := c.Generate(context.Background(), "sys", "usr", LLMOpts{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.PromptTokens != 42 {
		t.Errorf("PromptTokens = %d, want 42", res.PromptTokens)
	}
	if res.CompletionTokens != 7 {
		t.Errorf("CompletionTokens = %d, want 7", res.CompletionTokens)
	}
}

func TestCachedLLMClient_Model_Delegates(t *testing.T) {
	inner := &stubCountLLM{}
	c := NewCachedLLMClient(inner, nil, "test:")
	if c.Model() != "stub-model" {
		t.Errorf("Model() = %q, want %q", c.Model(), "stub-model")
	}
}

func TestCachedLLMClient_PriceDelegates(t *testing.T) {
	inner := &stubCountLLM{}
	c := NewCachedLLMClient(inner, nil, "test:")
	in, out := c.PriceMicrosPer1KTokens()
	if in != 10 || out != 20 {
		t.Errorf("PriceMicrosPer1KTokens() = (%d, %d), want (10, 20)", in, out)
	}
}

func TestCachedLLMClient_CacheKey_DiffersOnModel(t *testing.T) {
	inner1 := &stubCountLLM{}
	inner2 := &stubCountLLM{modelName: "other-model"}

	// Same prefix; only the underlying model differs.
	c1 := NewCachedLLMClient(inner1, nil, "prefix:")
	c2 := NewCachedLLMClient(inner2, nil, "prefix:")

	key1 := c1.cacheKey("sys", "usr", LLMOpts{})
	key2 := c2.cacheKey("sys", "usr", LLMOpts{})

	if key1 == key2 {
		t.Error("expected different cache keys for different models, got same key")
	}
}

// TestCachedLLMClient_GobRoundTrip verifies that the gob encoding used for Redis
// storage faithfully preserves integer token counts. The test bypasses Redis to
// exercise the encode/decode path in isolation without external infrastructure.
func TestCachedLLMClient_GobRoundTrip(t *testing.T) {
	want := LLMResult{Content: "x", PromptTokens: 42, CompletionTokens: 7}

	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(want); err != nil {
		t.Fatalf("gob encode: %v", err)
	}

	var got LLMResult
	if err := gob.NewDecoder(bytes.NewReader(buf.Bytes())).Decode(&got); err != nil {
		t.Fatalf("gob decode: %v", err)
	}

	if got.Content != want.Content {
		t.Errorf("Content = %q, want %q", got.Content, want.Content)
	}
	if got.PromptTokens != want.PromptTokens {
		t.Errorf("PromptTokens = %d, want %d", got.PromptTokens, want.PromptTokens)
	}
	if got.CompletionTokens != want.CompletionTokens {
		t.Errorf("CompletionTokens = %d, want %d", got.CompletionTokens, want.CompletionTokens)
	}
}

func TestCachedLLMClient_CacheKey_DiffersOnOpts(t *testing.T) {
	inner := &stubCountLLM{}
	c := NewCachedLLMClient(inner, nil, "test:")

	key1 := c.cacheKey("sys", "usr", LLMOpts{Temperature: 0.0, MaxTokens: 100, JSONMode: false})
	key2 := c.cacheKey("sys", "usr", LLMOpts{Temperature: 0.5, MaxTokens: 100, JSONMode: false})
	key3 := c.cacheKey("sys", "usr", LLMOpts{Temperature: 0.0, MaxTokens: 100, JSONMode: true})
	key4 := c.cacheKey("sys", "usr", LLMOpts{Temperature: 0.0, MaxTokens: 200, JSONMode: false})

	if key1 == key2 {
		t.Error("expected different keys for different temperatures")
	}
	if key1 == key3 {
		t.Error("expected different keys for different JSONMode values")
	}
	if key1 == key4 {
		t.Error("expected different keys for different MaxTokens values")
	}
}
