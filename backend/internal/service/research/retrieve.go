package research

import (
	"context"
	"time"

	"papertrader/internal/data"
)

// EmbeddingsSearcher is the subset of data.EmbeddingsStore used by RetrievalService.
// Keeping this interface small lets tests inject a stub without spinning up a DB.
type EmbeddingsSearcher interface {
	Search(ctx context.Context, vec []float32, opts data.SearchOpts) ([]data.ChunkHit, error)
}

// RetrievalService embeds a query and returns matching chunks from the vector store.
type RetrievalService struct {
	embedder   Embedder
	embeddings EmbeddingsSearcher
}

func NewRetrievalService(embedder Embedder, embeddings EmbeddingsSearcher) *RetrievalService {
	return &RetrievalService{embedder: embedder, embeddings: embeddings}
}

// RetrieveOpts controls retrieval behaviour.
type RetrieveOpts struct {
	Symbols     []string
	SourceTypes []string
	Since       time.Time
	K           int     // default 8
	MinScore    float64 // default 0.40
}

// Retrieve embeds query, runs ANN search, and filters results below MinScore.
func (r *RetrievalService) Retrieve(ctx context.Context, query string, opts RetrieveOpts) ([]data.ChunkHit, error) {
	k := opts.K
	if k <= 0 {
		k = 8
	}
	minScore := opts.MinScore
	if minScore <= 0 {
		minScore = 0.40
	}

	vec, _, err := r.embedder.EmbedQuery(ctx, query)
	if err != nil {
		return nil, err
	}

	searchOpts := data.SearchOpts{
		Symbols:     opts.Symbols,
		SourceTypes: opts.SourceTypes,
		K:           k,
	}
	if !opts.Since.IsZero() {
		searchOpts.Since = &opts.Since
	}

	hits, err := r.embeddings.Search(ctx, vec, searchOpts)
	if err != nil {
		return nil, err
	}

	filtered := hits[:0]
	for _, h := range hits {
		if h.Score >= minScore {
			filtered = append(filtered, h)
		}
	}
	return filtered, nil
}
