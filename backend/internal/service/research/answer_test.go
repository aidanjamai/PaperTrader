package research

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"papertrader/internal/data"
)

// ---- stubs ----

type stubRetriever struct {
	hits    []data.ChunkHit
	err     error
	called  int
}

func (s *stubRetriever) Retrieve(_ context.Context, _ string, _ RetrieveOpts) ([]data.ChunkHit, error) {
	s.called++
	return s.hits, s.err
}

type stubLLM struct {
	result LLMResult
	err    error
}

func (s *stubLLM) Generate(_ context.Context, _, _ string, _ LLMOpts) (LLMResult, error) {
	return s.result, s.err
}
func (s *stubLLM) Model() string                        { return "stub-llm" }
func (s *stubLLM) PriceMicrosPer1KTokens() (int, int) { return 59, 79 }

type capturedQuery struct {
	q data.ResearchQuery
}

type stubQueriesStore struct {
	inserts []capturedQuery
}

func (s *stubQueriesStore) Insert(_ context.Context, q data.ResearchQuery) error {
	s.inserts = append(s.inserts, capturedQuery{q: q})
	return nil
}

// ---- helpers ----

func makeHits(n int, score float64) []data.ChunkHit {
	t := time.Now()
	hits := make([]data.ChunkHit, n)
	for i := range hits {
		hits[i] = data.ChunkHit{
			ChunkID:   fmt.Sprintf("chunk-%d", i+1),
			DocumentID: fmt.Sprintf("doc-%d", i+1),
			Text:      "some source text about financials",
			SourceURL: "https://example.com/filing",
			Symbol:    "AAPL",
			FiledAt:   &t,
			Score:     score,
		}
	}
	return hits
}

func newAnswerSvc(ret retriever, llm LLMClient, qs queriesStore) *AnswerService {
	return NewAnswerService(ret, llm, qs, nil, nil)
}

// ---- tests ----

func TestAsk_Refuses_NoSources(t *testing.T) {
	ret := &stubRetriever{hits: []data.ChunkHit{}}
	qs := &stubQueriesStore{}
	svc := newAnswerSvc(ret, &stubLLM{}, qs)

	answer, err := svc.Ask(context.Background(), "user1", "What is AAPL revenue?", AskOpts{})
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if !answer.Refused {
		t.Error("expected Refused=true")
	}
	if answer.RefusalReason != "no_sources" {
		t.Errorf("RefusalReason = %q, want %q", answer.RefusalReason, "no_sources")
	}
	if len(qs.inserts) != 1 {
		t.Fatalf("expected 1 inserted row, got %d", len(qs.inserts))
	}
	if !qs.inserts[0].q.Refused {
		t.Error("persisted row should have Refused=true")
	}
}

func TestAsk_Refuses_LowScore(t *testing.T) {
	ret := &stubRetriever{hits: makeHits(2, 0.30)} // all below default minScore 0.55
	qs := &stubQueriesStore{}
	svc := newAnswerSvc(ret, &stubLLM{}, qs)

	answer, err := svc.Ask(context.Background(), "user1", "What is AAPL revenue?", AskOpts{MinScore: 0.55})
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if !answer.Refused {
		t.Error("expected Refused=true for low-score hits")
	}
	if answer.RefusalReason != "no_sources" {
		t.Errorf("RefusalReason = %q, want %q", answer.RefusalReason, "no_sources")
	}
	if len(qs.inserts) != 1 {
		t.Fatalf("expected 1 inserted row, got %d", len(qs.inserts))
	}
}

func TestAsk_Refuses_ForwardLooking(t *testing.T) {
	ret := &stubRetriever{}
	qs := &stubQueriesStore{}
	svc := newAnswerSvc(ret, &stubLLM{}, qs)

	answer, err := svc.Ask(context.Background(), "user1", "should I buy NVDA next week", AskOpts{})
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if !answer.Refused {
		t.Error("expected Refused=true for forward-looking query")
	}
	if answer.RefusalReason != "forward_looking" {
		t.Errorf("RefusalReason = %q, want %q", answer.RefusalReason, "forward_looking")
	}
	if ret.called != 0 {
		t.Errorf("retriever should NOT be called for forward-looking queries, called %d times", ret.called)
	}
	if len(qs.inserts) != 1 {
		t.Fatalf("expected 1 persisted row, got %d", len(qs.inserts))
	}
}

func TestAsk_Refuses_OutOfScope(t *testing.T) {
	hits := makeHits(2, 0.80)
	ret := &stubRetriever{hits: hits}
	qs := &stubQueriesStore{}
	llm := &stubLLM{result: LLMResult{Content: "OUT_OF_SCOPE", PromptTokens: 50, CompletionTokens: 1}}
	svc := newAnswerSvc(ret, llm, qs)

	answer, err := svc.Ask(context.Background(), "user1", "What is AAPL gross margin?", AskOpts{})
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if !answer.Refused {
		t.Error("expected Refused=true for OUT_OF_SCOPE")
	}
	if answer.RefusalReason != "out_of_scope" {
		t.Errorf("RefusalReason = %q, want %q", answer.RefusalReason, "out_of_scope")
	}
}

func TestAsk_Refuses_InsufficientSources(t *testing.T) {
	hits := makeHits(2, 0.80)
	ret := &stubRetriever{hits: hits}
	qs := &stubQueriesStore{}
	llm := &stubLLM{result: LLMResult{Content: "INSUFFICIENT_SOURCES", PromptTokens: 50, CompletionTokens: 1}}
	svc := newAnswerSvc(ret, llm, qs)

	answer, err := svc.Ask(context.Background(), "user1", "What were AAPL's 2030 earnings?", AskOpts{})
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if !answer.Refused {
		t.Error("expected Refused=true for INSUFFICIENT_SOURCES")
	}
	if answer.RefusalReason != "no_sources" {
		t.Errorf("RefusalReason = %q, want %q", answer.RefusalReason, "no_sources")
	}
}

func TestAsk_Refuses_HallucinatedCitation(t *testing.T) {
	hits := makeHits(2, 0.80)
	ret := &stubRetriever{hits: hits}
	qs := &stubQueriesStore{}

	badID := "chunk-999"
	llmJSON := `{"answer":"revenue was X [1]","used_chunk_ids":["` + badID + `"]}`
	llm := &stubLLM{result: LLMResult{Content: llmJSON, PromptTokens: 50, CompletionTokens: 20}}
	svc := newAnswerSvc(ret, llm, qs)

	answer, err := svc.Ask(context.Background(), "user1", "What is AAPL revenue?", AskOpts{})
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if !answer.Refused {
		t.Error("expected Refused=true for hallucinated citation")
	}
	if answer.RefusalReason != "hallucinated_citation" {
		t.Errorf("RefusalReason = %q, want %q", answer.RefusalReason, "hallucinated_citation")
	}
}

func TestAsk_Refuses_ModelError(t *testing.T) {
	hits := makeHits(2, 0.80)
	ret := &stubRetriever{hits: hits}
	qs := &stubQueriesStore{}
	llm := &stubLLM{result: LLMResult{Content: "not valid json at all", PromptTokens: 40, CompletionTokens: 5}}
	svc := newAnswerSvc(ret, llm, qs)

	answer, err := svc.Ask(context.Background(), "user1", "What is AAPL revenue?", AskOpts{})
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if !answer.Refused {
		t.Error("expected Refused=true for invalid JSON from model")
	}
	if answer.RefusalReason != "model_error" {
		t.Errorf("RefusalReason = %q, want %q", answer.RefusalReason, "model_error")
	}
}

func TestAsk_HappyPath(t *testing.T) {
	hits := makeHits(3, 0.85)
	ret := &stubRetriever{hits: hits}
	qs := &stubQueriesStore{}

	usedIDs := []string{hits[0].ChunkID, hits[1].ChunkID, hits[2].ChunkID}
	usedIDsJSON, _ := json.Marshal(usedIDs)
	llmJSON := `{"answer":"revenue was $X [1][2][3]","used_chunk_ids":` + string(usedIDsJSON) + `}`
	llm := &stubLLM{result: LLMResult{Content: llmJSON, PromptTokens: 200, CompletionTokens: 50}}
	svc := newAnswerSvc(ret, llm, qs)

	answer, err := svc.Ask(context.Background(), "user1", "What is AAPL revenue?", AskOpts{})
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if answer.Refused {
		t.Errorf("expected Refused=false, got RefusalReason=%q", answer.RefusalReason)
	}
	if len(answer.Citations) != 3 {
		t.Errorf("expected 3 citations, got %d", len(answer.Citations))
	}
	if answer.LatencyMS < 0 {
		t.Error("expected LatencyMS >= 0")
	}
	if len(qs.inserts) != 1 {
		t.Fatalf("expected 1 persisted row, got %d", len(qs.inserts))
	}
	row := qs.inserts[0].q
	if row.PromptTokens == nil || *row.PromptTokens != 200 {
		t.Error("PromptTokens not persisted correctly")
	}
	if row.CompletionTokens == nil || *row.CompletionTokens != 50 {
		t.Error("CompletionTokens not persisted correctly")
	}
	if row.CostUSDMicros == nil || *row.CostUSDMicros <= 0 {
		t.Error("cost_usd_micros should be positive")
	}
}
