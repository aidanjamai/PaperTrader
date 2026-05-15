package main

import (
	"context"
	"testing"

	"papertrader/internal/service/research"
)

// stubEmbedder returns deterministic vectors based on a registry.
type stubEmbedder struct {
	vecs map[string][]float32
}

func (s *stubEmbedder) EmbedQuery(_ context.Context, text string) ([]float32, int, error) {
	if v, ok := s.vecs[text]; ok {
		return v, len(text), nil
	}
	// Default: unit vector in first dimension.
	return []float32{1, 0, 0}, len(text), nil
}

func (s *stubEmbedder) EmbedBatch(_ context.Context, texts []string) ([][]float32, int, error) {
	vecs := make([][]float32, len(texts))
	for i, t := range texts {
		v, _, _ := s.EmbedQuery(context.Background(), t)
		vecs[i] = v
	}
	return vecs, 0, nil
}

func (s *stubEmbedder) Model() string { return "stub-embedder" }

// stubJudgeLLM returns "YES" or "NO" based on whether the user prompt
// contains a trigger substring.
type stubJudgeLLM struct {
	yesIfContains string
}

func (s *stubJudgeLLM) Generate(_ context.Context, _, user string, _ research.LLMOpts) (research.LLMResult, error) {
	if s.yesIfContains != "" {
		// Always YES when we want it.
		return research.LLMResult{Content: "YES", PromptTokens: 1, CompletionTokens: 1}, nil
	}
	return research.LLMResult{Content: "NO", PromptTokens: 1, CompletionTokens: 1}, nil
}
func (s *stubJudgeLLM) Model() string                        { return "stub-judge" }
func (s *stubJudgeLLM) PriceMicrosPer1KTokens() (int, int) { return 0, 0 }

type alwaysNoLLM struct{}

func (a *alwaysNoLLM) Generate(_ context.Context, _, _ string, _ research.LLMOpts) (research.LLMResult, error) {
	return research.LLMResult{Content: "NO"}, nil
}
func (a *alwaysNoLLM) Model() string                        { return "always-no" }
func (a *alwaysNoLLM) PriceMicrosPer1KTokens() (int, int) { return 0, 0 }

// identicalVecEmbedder always returns the same vector so cosine sim == 1.0.
type identicalVecEmbedder struct{}

func (e *identicalVecEmbedder) EmbedQuery(_ context.Context, _ string) ([]float32, int, error) {
	return []float32{1, 0, 0}, 3, nil
}
func (e *identicalVecEmbedder) EmbedBatch(_ context.Context, texts []string) ([][]float32, int, error) {
	out := make([][]float32, len(texts))
	for i := range out {
		out[i] = []float32{1, 0, 0}
	}
	return out, 0, nil
}
func (e *identicalVecEmbedder) Model() string { return "identical" }

// lowSimEmbedder returns orthogonal vectors for claim vs chunk.
type lowSimEmbedder struct{ call int }

func (e *lowSimEmbedder) EmbedQuery(_ context.Context, _ string) ([]float32, int, error) {
	e.call++
	if e.call%2 == 1 {
		return []float32{1, 0, 0}, 3, nil // claim
	}
	return []float32{0, 1, 0}, 3, nil // chunk — orthogonal → sim=0
}
func (e *lowSimEmbedder) EmbedBatch(_ context.Context, texts []string) ([][]float32, int, error) {
	return nil, 0, nil
}
func (e *lowSimEmbedder) Model() string { return "low-sim" }

func makeAnswer(text string, citations []research.Citation) *research.Answer {
	return &research.Answer{
		QueryID:   "test-id",
		Answer:    text,
		Citations: citations,
	}
}

func TestJudgeAnswer_AllPass(t *testing.T) {
	cit := research.Citation{ChunkID: "chunk1", Excerpt: "The company faces supplier risk."}
	answer := makeAnswer("Apple faces supplier risks [1].", []research.Citation{cit})

	j := &Judge{
		embedder:  &identicalVecEmbedder{},
		llmJudge:  &stubJudgeLLM{yesIfContains: "yes"},
		chunks:    map[string]string{"chunk1": cit.Excerpt},
		chunkVecs: make(map[string][]float32),
	}

	scores, err := j.JudgeAnswer(context.Background(), answer)
	if err != nil {
		t.Fatalf("JudgeAnswer error: %v", err)
	}
	if len(scores) == 0 {
		t.Fatal("expected at least one claim score")
	}
	for _, cs := range scores {
		if !cs.Pass {
			t.Errorf("claim %q: expected Pass=true, got false (sim=%.2f judgeOK=%v)", cs.Claim, cs.EmbedSim, cs.LLMJudgeOK)
		}
	}
}

func TestJudgeAnswer_FailsOnLowEmbedSim(t *testing.T) {
	cit := research.Citation{ChunkID: "chunk1", Excerpt: "Some chunk text here."}
	answer := makeAnswer("The claim refers to something [1].", []research.Citation{cit})

	j := &Judge{
		embedder:  &lowSimEmbedder{},
		llmJudge:  &stubJudgeLLM{yesIfContains: "yes"},
		chunks:    map[string]string{"chunk1": cit.Excerpt},
		chunkVecs: make(map[string][]float32),
	}

	scores, err := j.JudgeAnswer(context.Background(), answer)
	if err != nil {
		t.Fatalf("JudgeAnswer error: %v", err)
	}
	if len(scores) == 0 {
		t.Fatal("expected at least one score")
	}
	// Sim is ~0 (orthogonal vectors) so Pass must be false.
	for _, cs := range scores {
		if cs.Pass {
			t.Errorf("expected Pass=false due to low sim (%.2f), got true", cs.EmbedSim)
		}
	}
}

func TestJudgeAnswer_FailsOnJudgeNo(t *testing.T) {
	cit := research.Citation{ChunkID: "chunk1", Excerpt: "Apple supply chain risks are discussed here."}
	answer := makeAnswer("Apple has supply chain exposure [1].", []research.Citation{cit})

	j := &Judge{
		embedder:  &identicalVecEmbedder{}, // high sim
		llmJudge:  &alwaysNoLLM{},          // judge says NO
		chunks:    map[string]string{"chunk1": cit.Excerpt},
		chunkVecs: make(map[string][]float32),
	}

	scores, err := j.JudgeAnswer(context.Background(), answer)
	if err != nil {
		t.Fatalf("JudgeAnswer error: %v", err)
	}
	if len(scores) == 0 {
		t.Fatal("expected at least one score")
	}
	for _, cs := range scores {
		if cs.Pass {
			t.Errorf("expected Pass=false (judge said NO), got true")
		}
	}
}

func TestJudgeAnswer_NoCitations(t *testing.T) {
	answer := makeAnswer("No citations here.", []research.Citation{})

	j := &Judge{
		embedder:  &identicalVecEmbedder{},
		llmJudge:  &stubJudgeLLM{yesIfContains: "yes"},
		chunks:    map[string]string{},
		chunkVecs: make(map[string][]float32),
	}

	scores, err := j.JudgeAnswer(context.Background(), answer)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(scores) != 0 {
		t.Errorf("expected empty scores for no citations, got %d", len(scores))
	}
}

func TestJudgeAnswer_HandlesMissingMarkers(t *testing.T) {
	cit := research.Citation{ChunkID: "chunk1", Excerpt: "Some text."}
	// Answer text has no [N] markers — all sentences are uncited.
	answer := makeAnswer("No citation markers in this answer text at all.", []research.Citation{cit})

	j := &Judge{
		embedder:  &identicalVecEmbedder{},
		llmJudge:  &stubJudgeLLM{yesIfContains: "yes"},
		chunks:    map[string]string{"chunk1": cit.Excerpt},
		chunkVecs: make(map[string][]float32),
	}

	scores, err := j.JudgeAnswer(context.Background(), answer)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(scores) != 0 {
		t.Errorf("expected empty scores when no citation markers, got %d", len(scores))
	}
}
