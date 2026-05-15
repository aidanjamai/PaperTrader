package research

import (
	"context"
	"testing"
	"time"

	"papertrader/internal/data"
)

// stubEmbedder always returns a fixed vector.
type stubEmbedder struct {
	vec []float32
}

func (s *stubEmbedder) EmbedQuery(_ context.Context, _ string) ([]float32, int, error) {
	return s.vec, 10, nil
}
func (s *stubEmbedder) EmbedBatch(_ context.Context, texts []string) ([][]float32, int, error) {
	out := make([][]float32, len(texts))
	for i := range texts {
		out[i] = s.vec
	}
	return out, len(texts) * 10, nil
}
func (s *stubEmbedder) Model() string { return "stub" }

// stubSearcher returns a canned list of hits, honouring K.
type stubSearcher struct {
	hits []data.ChunkHit
}

func (s *stubSearcher) Search(_ context.Context, _ []float32, opts data.SearchOpts) ([]data.ChunkHit, error) {
	k := opts.K
	if k <= 0 || k > len(s.hits) {
		k = len(s.hits)
	}
	return s.hits[:k], nil
}

func TestRetrieve_MinScoreFiltering(t *testing.T) {
	filedAt := time.Now()
	hits := []data.ChunkHit{
		{ChunkID: "c1", Score: 0.90, FiledAt: &filedAt},
		{ChunkID: "c2", Score: 0.60, FiledAt: &filedAt},
		{ChunkID: "c3", Score: 0.40, FiledAt: &filedAt}, // below threshold
	}
	svc := NewRetrievalService(&stubEmbedder{vec: []float32{1, 0}}, &stubSearcher{hits: hits})

	got, err := svc.Retrieve(context.Background(), "any query", RetrieveOpts{MinScore: 0.55, K: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 hits above MinScore 0.55, got %d", len(got))
	}
	for _, h := range got {
		if h.Score < 0.55 {
			t.Errorf("hit %s has score %.2f below MinScore", h.ChunkID, h.Score)
		}
	}
}

func TestRetrieve_KHonored(t *testing.T) {
	filedAt := time.Now()
	hits := []data.ChunkHit{
		{ChunkID: "c1", Score: 0.95, FiledAt: &filedAt},
		{ChunkID: "c2", Score: 0.90, FiledAt: &filedAt},
		{ChunkID: "c3", Score: 0.85, FiledAt: &filedAt},
		{ChunkID: "c4", Score: 0.80, FiledAt: &filedAt},
		{ChunkID: "c5", Score: 0.75, FiledAt: &filedAt},
	}
	svc := NewRetrievalService(&stubEmbedder{vec: []float32{1, 0}}, &stubSearcher{hits: hits})

	got, err := svc.Retrieve(context.Background(), "query", RetrieveOpts{K: 3, MinScore: 0.0})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("K=3: expected 3 hits, got %d", len(got))
	}
}

func TestRetrieve_AllBelowMinScore(t *testing.T) {
	filedAt := time.Now()
	hits := []data.ChunkHit{
		{ChunkID: "c1", Score: 0.30, FiledAt: &filedAt},
		{ChunkID: "c2", Score: 0.20, FiledAt: &filedAt},
	}
	svc := NewRetrievalService(&stubEmbedder{vec: []float32{1, 0}}, &stubSearcher{hits: hits})

	got, err := svc.Retrieve(context.Background(), "query", RetrieveOpts{MinScore: 0.55, K: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("expected 0 hits, got %d", len(got))
	}
}
