package research

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func voyageResponse(t *testing.T, n int) []byte {
	t.Helper()
	type embedding struct {
		Object    string    `json:"object"`
		Embedding []float32 `json:"embedding"`
		Index     int       `json:"index"`
	}
	type usage struct {
		TotalTokens int `json:"total_tokens"`
	}
	type response struct {
		Object string      `json:"object"`
		Data   []embedding `json:"data"`
		Model  string      `json:"model"`
		Usage  usage       `json:"usage"`
	}

	data := make([]embedding, n)
	for i := range data {
		vec := make([]float32, 1024)
		vec[i%1024] = 1.0
		data[i] = embedding{Object: "embedding", Embedding: vec, Index: i}
	}
	b, err := json.Marshal(response{
		Object: "list",
		Data:   data,
		Model:  "voyage-finance-2",
		Usage:  usage{TotalTokens: n * 10},
	})
	if err != nil {
		t.Fatalf("marshal voyage response: %v", err)
	}
	return b
}

func TestVoyageEmbedder_EmbedQuery_AuthHeader(t *testing.T) {
	const apiKey = "test-key-abc"
	var gotAuth string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.Write(voyageResponse(t, 1))
	}))
	defer srv.Close()

	e := &VoyageEmbedder{
		httpClient: srv.Client(),
		apiKey:     apiKey,
		model:      "voyage-finance-2",
	}
	// Point at the test server by temporarily replacing the URL via the client transport.
	// We reach into callAPI via EmbedQuery; override the endpoint by wrapping the transport
	// so requests to voyageai.com are redirected to the test server.
	e.httpClient = &http.Client{
		Transport: redirectTransport(srv.URL),
	}

	_, _, err := e.EmbedQuery(context.Background(), "hello")
	if err != nil {
		t.Fatalf("EmbedQuery: %v", err)
	}
	if gotAuth != "Bearer "+apiKey {
		t.Errorf("Authorization header = %q, want %q", gotAuth, "Bearer "+apiKey)
	}
}

func TestVoyageEmbedder_EmbedQuery_InputType(t *testing.T) {
	var gotBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(voyageResponse(t, 1))
	}))
	defer srv.Close()

	e := &VoyageEmbedder{
		httpClient: &http.Client{Transport: redirectTransport(srv.URL)},
		apiKey:     "k",
		model:      "voyage-finance-2",
	}

	if _, _, err := e.EmbedQuery(context.Background(), "query text"); err != nil {
		t.Fatalf("EmbedQuery: %v", err)
	}
	if gotBody["input_type"] != "query" {
		t.Errorf("EmbedQuery input_type = %v, want %q", gotBody["input_type"], "query")
	}
	if gotBody["model"] != "voyage-finance-2" {
		t.Errorf("EmbedQuery model = %v, want %q", gotBody["model"], "voyage-finance-2")
	}
}

func TestVoyageEmbedder_EmbedBatch_InputType(t *testing.T) {
	var gotBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(voyageResponse(t, 2))
	}))
	defer srv.Close()

	e := &VoyageEmbedder{
		httpClient: &http.Client{Transport: redirectTransport(srv.URL)},
		apiKey:     "k",
		model:      "voyage-finance-2",
	}

	if _, _, err := e.EmbedBatch(context.Background(), []string{"doc one", "doc two"}); err != nil {
		t.Fatalf("EmbedBatch: %v", err)
	}
	if gotBody["input_type"] != "document" {
		t.Errorf("EmbedBatch input_type = %v, want %q", gotBody["input_type"], "document")
	}
}

func TestVoyageEmbedder_URLDoesNotContainKey(t *testing.T) {
	const apiKey = "secret-voyage-key"
	var gotURL string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		w.Write(voyageResponse(t, 1))
	}))
	defer srv.Close()

	e := &VoyageEmbedder{
		httpClient: &http.Client{Transport: redirectTransport(srv.URL)},
		apiKey:     apiKey,
		model:      "voyage-finance-2",
	}

	if _, _, err := e.EmbedQuery(context.Background(), "test"); err != nil {
		t.Fatalf("EmbedQuery: %v", err)
	}
	if strings.Contains(gotURL, apiKey) {
		t.Errorf("request URL contains API key: %q", gotURL)
	}
}

func TestVoyageEmbedder_ErrorDoesNotLeakKey(t *testing.T) {
	const apiKey = "super-secret-key"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
	}))
	defer srv.Close()

	e := &VoyageEmbedder{
		httpClient: &http.Client{Transport: redirectTransport(srv.URL)},
		apiKey:     apiKey,
		model:      "voyage-finance-2",
	}

	_, _, err := e.EmbedQuery(context.Background(), "test")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if strings.Contains(err.Error(), apiKey) {
		t.Errorf("error message contains API key: %v", err)
	}
}

// redirectTransport rewrites every request's host to the given base URL so
// the real voyageai.com hostname is never contacted during tests.
type redirectTransport string

func (base redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req2 := req.Clone(req.Context())
	req2.URL.Scheme = "http"
	req2.URL.Host = strings.TrimPrefix(string(base), "http://")
	return http.DefaultTransport.RoundTrip(req2)
}
