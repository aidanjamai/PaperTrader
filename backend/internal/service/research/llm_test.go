package research

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func groqResponse(t *testing.T, content string, promptTokens, completionTokens int) []byte {
	t.Helper()
	type message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	type choice struct {
		Message message `json:"message"`
	}
	type usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	}
	type response struct {
		Choices []choice `json:"choices"`
		Usage   usage    `json:"usage"`
	}
	b, err := json.Marshal(response{
		Choices: []choice{{Message: message{Role: "assistant", Content: content}}},
		Usage:   usage{PromptTokens: promptTokens, CompletionTokens: completionTokens},
	})
	if err != nil {
		t.Fatalf("marshal groq response: %v", err)
	}
	return b
}

func TestGroqClient_AuthHeader(t *testing.T) {
	const apiKey = "gsk_test-key-abc"
	var gotAuth string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.Write(groqResponse(t, `{"answer":"ok","used_chunk_ids":[]}`, 10, 5))
	}))
	defer srv.Close()

	c := &GroqClient{
		httpClient: &http.Client{Transport: redirectTransport(srv.URL)},
		apiKey:     apiKey,
		model:      "llama-3.3-70b-versatile",
	}

	if _, err := c.Generate(context.Background(), "system", "user", LLMOpts{JSONMode: true}); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if gotAuth != "Bearer "+apiKey {
		t.Errorf("Authorization header = %q, want %q", gotAuth, "Bearer "+apiKey)
	}
}

func TestGroqClient_JSONMode_Present(t *testing.T) {
	var gotBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(groqResponse(t, `{"answer":"ok","used_chunk_ids":[]}`, 5, 3))
	}))
	defer srv.Close()

	c := &GroqClient{
		httpClient: &http.Client{Transport: redirectTransport(srv.URL)},
		apiKey:     "k",
		model:      "llama-3.3-70b-versatile",
	}

	if _, err := c.Generate(context.Background(), "sys", "usr", LLMOpts{JSONMode: true}); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	rf, ok := gotBody["response_format"]
	if !ok {
		t.Fatal("response_format missing from request body when JSONMode=true")
	}
	rfMap, ok := rf.(map[string]any)
	if !ok || rfMap["type"] != "json_object" {
		t.Errorf("response_format = %v, want {type: json_object}", rf)
	}
}

func TestGroqClient_JSONMode_Absent(t *testing.T) {
	var gotBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(groqResponse(t, "plain text answer", 5, 3))
	}))
	defer srv.Close()

	c := &GroqClient{
		httpClient: &http.Client{Transport: redirectTransport(srv.URL)},
		apiKey:     "k",
		model:      "llama-3.3-70b-versatile",
	}

	if _, err := c.Generate(context.Background(), "sys", "usr", LLMOpts{JSONMode: false}); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if _, present := gotBody["response_format"]; present {
		t.Error("response_format should be absent when JSONMode=false")
	}
}

func TestGroqClient_URLDoesNotContainKey(t *testing.T) {
	const apiKey = "gsk_super-secret-key"
	var gotURL string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		w.Write(groqResponse(t, "hi", 1, 1))
	}))
	defer srv.Close()

	c := &GroqClient{
		httpClient: &http.Client{Transport: redirectTransport(srv.URL)},
		apiKey:     apiKey,
		model:      "llama-3.3-70b-versatile",
	}

	if _, err := c.Generate(context.Background(), "s", "u", LLMOpts{}); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if strings.Contains(gotURL, apiKey) {
		t.Errorf("request URL contains API key: %q", gotURL)
	}
}

func TestGroqClient_ErrorDoesNotLeakKey(t *testing.T) {
	const apiKey = "gsk_leaked-if-visible"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
	}))
	defer srv.Close()

	c := &GroqClient{
		httpClient: &http.Client{Transport: redirectTransport(srv.URL)},
		apiKey:     apiKey,
		model:      "llama-3.3-70b-versatile",
	}

	_, err := c.Generate(context.Background(), "s", "u", LLMOpts{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if strings.Contains(err.Error(), apiKey) {
		t.Errorf("error message contains API key: %v", err)
	}
}

func TestGroqClient_UsageParsed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(groqResponse(t, "answer", 123, 456))
	}))
	defer srv.Close()

	c := &GroqClient{
		httpClient: &http.Client{Transport: redirectTransport(srv.URL)},
		apiKey:     "k",
		model:      "llama-3.3-70b-versatile",
	}

	res, err := c.Generate(context.Background(), "s", "u", LLMOpts{})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if res.PromptTokens != 123 {
		t.Errorf("PromptTokens = %d, want 123", res.PromptTokens)
	}
	if res.CompletionTokens != 456 {
		t.Errorf("CompletionTokens = %d, want 456", res.CompletionTokens)
	}
}
