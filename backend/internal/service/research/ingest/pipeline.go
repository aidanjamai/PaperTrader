package ingest

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"papertrader/internal/data"
	"papertrader/internal/service/research"
)

// filingFetcher is the EdgarClient subset used by Pipeline.
type filingFetcher interface {
	ResolveCIK(ctx context.Context, symbol string) (string, error)
	FetchRecentFilings(ctx context.Context, cik string, formTypes []string, n int) ([]Filing, error)
	FetchFilingText(ctx context.Context, f Filing) (string, error)
}

// documentStore is the DocumentsStore subset used by Pipeline.
type documentStore interface {
	GetByID(ctx context.Context, id string) (*data.Document, error)
	Upsert(ctx context.Context, doc data.Document) error
}

// chunkStore is the ChunksStore subset used by Pipeline.
type chunkStore interface {
	BulkInsert(ctx context.Context, chunks []data.Chunk) error
	GetByDocumentID(ctx context.Context, docID string) ([]data.Chunk, error)
}

// embeddingStore is the EmbeddingsStore subset used by Pipeline.
type embeddingStore interface {
	Upsert(ctx context.Context, chunkID string, vec []float32, model string) error
	Exists(ctx context.Context, chunkID string) (bool, error)
}

// Pipeline orchestrates the full ingest flow: fetch → chunk → embed → store.
// Processing is deliberately serial (one filing at a time) to keep the RAM
// footprint predictable on the constrained e2-micro instance. A single 10-K
// can spike to ~80 MB while chunks are held in memory during embedding.
type Pipeline struct {
	edgar      filingFetcher
	embedder   research.Embedder
	docs       documentStore
	chunks     chunkStore
	embeddings embeddingStore
}

// IngestOpts controls ingest behaviour.
type IngestOpts struct {
	FormTypes  []string
	MaxFilings int
	Force      bool
}

// IngestResult summarises what changed.
type IngestResult struct {
	DocumentsAdded  int
	ChunksAdded     int
	EmbeddingsAdded int
	Skipped         int
}

func NewPipeline(
	edgar *EdgarClient,
	embedder research.Embedder,
	docs *data.DocumentsStore,
	chunks *data.ChunksStore,
	embeddings *data.EmbeddingsStore,
) *Pipeline {
	return &Pipeline{
		edgar:      edgar,
		embedder:   embedder,
		docs:       docs,
		chunks:     chunks,
		embeddings: embeddings,
	}
}

// IngestSymbol ingests recent SEC filings for one ticker symbol.
func (p *Pipeline) IngestSymbol(ctx context.Context, symbol string, opts IngestOpts) (*IngestResult, error) {
	formTypes := opts.FormTypes
	if len(formTypes) == 0 {
		formTypes = []string{"10-K", "10-Q", "8-K"}
	}
	maxFilings := opts.MaxFilings
	if maxFilings <= 0 {
		maxFilings = 5
	}

	cik, err := p.edgar.ResolveCIK(ctx, symbol)
	if err != nil {
		return nil, fmt.Errorf("ingest %s: %w", symbol, err)
	}

	filings, err := p.edgar.FetchRecentFilings(ctx, cik, formTypes, maxFilings)
	if err != nil {
		return nil, fmt.Errorf("ingest %s: %w", symbol, err)
	}

	result := &IngestResult{}
	for _, f := range filings {
		if err := p.ingestFiling(ctx, symbol, f, opts, result); err != nil {
			slog.Warn("ingest: filing failed, continuing", "url", f.URL, "err", err)
		}
	}
	return result, nil
}

func (p *Pipeline) ingestFiling(ctx context.Context, symbol string, f Filing, opts IngestOpts, result *IngestResult) error {
	// Defense in depth: source_url is rendered as a link in the frontend, so
	// reject any scheme that could execute in the browser. The DB also has a
	// CHECK constraint, but failing here gives a clearer error than a constraint
	// violation deep in the stack.
	if !isSafeHTTPURL(f.URL) {
		return fmt.Errorf("ingest: refusing non-http(s) source_url: %s", f.URL)
	}

	docID := docIDFromURL(f.URL)

	if !opts.Force {
		existing, err := p.docs.GetByID(ctx, docID)
		if err != nil {
			return fmt.Errorf("check existing doc: %w", err)
		}

		if existing != nil {
			// Document exists — check whether all chunks have embeddings. If any
			// are missing this is a partial failure from a prior run; backfill only
			// the unembedded chunks rather than skipping the filing entirely.
			chunks, err := p.chunks.GetByDocumentID(ctx, docID)
			if err != nil {
				return fmt.Errorf("get chunks for backfill check: %w", err)
			}

			var toEmbed []data.Chunk
			for _, c := range chunks {
				exists, err := p.embeddings.Exists(ctx, c.ID)
				if err != nil {
					return fmt.Errorf("check embedding exists for %s: %w", c.ID, err)
				}
				if !exists {
					toEmbed = append(toEmbed, c)
				}
			}

			if len(toEmbed) == 0 {
				result.Skipped++
				slog.Info("ingest: skipping existing document (all chunks embedded)", "url", f.URL)
				return nil
			}

			slog.Info("ingest: backfilling missing embeddings", "url", f.URL, "count", len(toEmbed))
			if err := p.embedAndStore(ctx, toEmbed, false); err != nil {
				slog.Warn("ingest: backfill failed, continuing", "url", f.URL, "err", err)
				return nil
			}
			result.EmbeddingsAdded += len(toEmbed)
			return nil
		}
	}

	text, err := p.edgar.FetchFilingText(ctx, f)
	if err != nil {
		return fmt.Errorf("fetch text: %w", err)
	}

	rawChunks := ChunkText(text, ChunkOpts{TargetTokens: 500, OverlapTokens: 80})
	if len(rawChunks) == 0 {
		slog.Warn("ingest: no chunks produced", "url", f.URL)
		return nil
	}

	var filedAt *time.Time
	if f.FiledAt != "" {
		t, err := time.Parse("2006-01-02", f.FiledAt)
		if err == nil {
			filedAt = &t
		}
	}

	doc := data.Document{
		ID:         docID,
		SourceType: "sec_filing",
		SourceURL:  f.URL,
		Symbol:     symbol,
		Title:      fmt.Sprintf("%s %s", f.FormType, f.FiledAt),
		FiledAt:    filedAt,
		Metadata:   []byte(fmt.Sprintf(`{"form_type":%q,"cik":%q,"accession":%q}`, f.FormType, f.CIK, f.AccessionNumber)),
	}
	if err := p.docs.Upsert(ctx, doc); err != nil {
		return fmt.Errorf("upsert doc: %w", err)
	}
	result.DocumentsAdded++

	dataChunks := make([]data.Chunk, len(rawChunks))
	for i, c := range rawChunks {
		dataChunks[i] = data.Chunk{
			ID:         chunkID(docID, c.Index),
			DocumentID: docID,
			ChunkIndex: c.Index,
			Text:       c.Text,
			TokenCount: c.TokenCount,
			Section:    c.Section,
		}
	}
	if err := p.chunks.BulkInsert(ctx, dataChunks); err != nil {
		return fmt.Errorf("bulk insert chunks: %w", err)
	}
	result.ChunksAdded += len(dataChunks)

	if err := p.embedAndStore(ctx, dataChunks, opts.Force); err != nil {
		return err
	}
	result.EmbeddingsAdded += len(dataChunks)

	return nil
}

// embedAndStore embeds chunks in batches of 32 and upserts each embedding.
// When force is false it skips chunks that already have an embedding, protecting
// against double-work on partial-batch failures even on the new-document path.
// When force is true the existence check is skipped — the caller explicitly
// wants every chunk re-embedded.
func (p *Pipeline) embedAndStore(ctx context.Context, chunks []data.Chunk, force bool) error {
	const batchSize = 32

	for start := 0; start < len(chunks); start += batchSize {
		end := start + batchSize
		if end > len(chunks) {
			end = len(chunks)
		}
		batch := chunks[start:end]

		// Filter to chunks that still need embedding when not forcing.
		toEmbed := batch
		if !force {
			filtered := batch[:0:len(batch)]
			for _, c := range batch {
				exists, err := p.embeddings.Exists(ctx, c.ID)
				if err != nil {
					return fmt.Errorf("check embedding exists for %s: %w", c.ID, err)
				}
				if !exists {
					filtered = append(filtered, c)
				}
			}
			toEmbed = filtered
		}

		if len(toEmbed) == 0 {
			continue
		}

		texts := make([]string, len(toEmbed))
		for i, c := range toEmbed {
			texts[i] = c.Text
		}

		vecs, _, err := p.embedder.EmbedBatch(ctx, texts)
		if err != nil {
			return fmt.Errorf("embed batch [%d:%d]: %w", start, end, err)
		}

		for i, c := range toEmbed {
			if err := p.embeddings.Upsert(ctx, c.ID, vecs[i], p.embedder.Model()); err != nil {
				return fmt.Errorf("upsert embedding %s: %w", c.ID, err)
			}
		}
	}

	return nil
}

// isSafeHTTPURL returns true only for http:// and https:// URLs. Anything else
// (javascript:, data:, file:, schemeless) is rejected before the URL enters the
// documents table and, transitively, the frontend.
func isSafeHTTPURL(u string) bool {
	return strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://")
}

func docIDFromURL(url string) string {
	sum := sha256.Sum256([]byte(url))
	return fmt.Sprintf("%x", sum[:32])
}

func chunkID(docID string, index int) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s:%d", docID, index)))
	return fmt.Sprintf("%x", sum[:32])
}

// ChunkIDFromParts is exported for use in tests.
func ChunkIDFromParts(docID string, index int) string {
	return chunkID(docID, index)
}

// DocIDFromURL is exported for use in tests and the CLI.
func DocIDFromURL(url string) string {
	return docIDFromURL(url)
}
