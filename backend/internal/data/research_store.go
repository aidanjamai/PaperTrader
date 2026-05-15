package data

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"
)

// Document is one source filing or article stored before chunking.
type Document struct {
	ID         string
	SourceType string
	SourceURL  string
	Symbol     string
	Title      string
	FiledAt    *time.Time
	Metadata   []byte // raw JSONB
	FetchedAt  time.Time
}

// Chunk is a text fragment derived from a Document.
type Chunk struct {
	ID         string
	DocumentID string
	ChunkIndex int
	Text       string
	TokenCount int
	Section    string
}

// ChunkHit is a retrieval result combining embedding score with chunk/doc metadata.
type ChunkHit struct {
	ChunkID    string
	DocumentID string
	Text       string
	Section    string
	SourceURL  string
	Symbol     string
	FiledAt    *time.Time
	Score      float64
}

// SearchOpts controls the vector search query.
type SearchOpts struct {
	Symbols     []string
	SourceTypes []string
	Since       *time.Time
	K           int
}

// ResearchQuery is one row in the research_queries audit table.
type ResearchQuery struct {
	ID               string
	UserID           *string
	QueryText        string
	AnswerText       *string
	Refused          bool
	RefusalReason    *string
	Citations        []byte
	RetrievalMS      *int
	GenerationMS     *int
	TotalMS          *int
	EmbedTokens      *int
	PromptTokens     *int
	CompletionTokens *int
	CostUSDMicros    *int
	Model            *string
}

// ---- DocumentsStore ----

type DocumentsStore struct {
	db DBTX
}

func NewDocumentsStore(db DBTX) *DocumentsStore {
	return &DocumentsStore{db: db}
}

func (s *DocumentsStore) Upsert(ctx context.Context, doc Document) error {
	query := `
INSERT INTO documents (id, source_type, source_url, symbol, title, filed_at, metadata)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (source_url) DO UPDATE SET
    source_type = EXCLUDED.source_type,
    symbol      = EXCLUDED.symbol,
    title       = EXCLUDED.title,
    filed_at    = EXCLUDED.filed_at,
    metadata    = EXCLUDED.metadata,
    fetched_at  = CURRENT_TIMESTAMP`

	metadata := doc.Metadata
	if metadata == nil {
		metadata = []byte("{}")
	}

	_, err := s.db.ExecContext(ctx, query,
		doc.ID, doc.SourceType, doc.SourceURL,
		doc.Symbol, doc.Title, doc.FiledAt,
		metadata,
	)
	return err
}

func (s *DocumentsStore) GetByID(ctx context.Context, id string) (*Document, error) {
	query := `
SELECT id, source_type, source_url, symbol, title, filed_at, metadata, fetched_at
FROM documents WHERE id = $1`
	return s.scanDocument(ctx, query, id)
}

func (s *DocumentsStore) GetBySourceURL(ctx context.Context, url string) (*Document, error) {
	query := `
SELECT id, source_type, source_url, symbol, title, filed_at, metadata, fetched_at
FROM documents WHERE source_url = $1`
	return s.scanDocument(ctx, query, url)
}

func (s *DocumentsStore) scanDocument(ctx context.Context, query string, arg interface{}) (*Document, error) {
	var d Document
	var symbol, title sql.NullString
	var filedAt sql.NullTime
	err := s.db.QueryRowContext(ctx, query, arg).Scan(
		&d.ID, &d.SourceType, &d.SourceURL,
		&symbol, &title, &filedAt,
		&d.Metadata, &d.FetchedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if symbol.Valid {
		d.Symbol = symbol.String
	}
	if title.Valid {
		d.Title = title.String
	}
	if filedAt.Valid {
		d.FiledAt = &filedAt.Time
	}
	return &d, nil
}

// ---- ChunksStore ----

type ChunksStore struct {
	db DBTX
}

func NewChunksStore(db DBTX) *ChunksStore {
	return &ChunksStore{db: db}
}

// BulkInsert inserts all chunks in a single multi-row statement.
// ON CONFLICT DO NOTHING keeps this idempotent on re-ingest.
func (s *ChunksStore) BulkInsert(ctx context.Context, chunks []Chunk) error {
	if len(chunks) == 0 {
		return nil
	}

	placeholders := make([]string, 0, len(chunks))
	args := make([]interface{}, 0, len(chunks)*5)
	idx := 1
	for _, c := range chunks {
		placeholders = append(placeholders, fmt.Sprintf("($%d,$%d,$%d,$%d,$%d,$%d)",
			idx, idx+1, idx+2, idx+3, idx+4, idx+5))
		args = append(args, c.ID, c.DocumentID, c.ChunkIndex, c.Text, c.TokenCount, c.Section)
		idx += 6
	}

	query := `INSERT INTO chunks (id, document_id, chunk_index, text, token_count, section)
VALUES ` + strings.Join(placeholders, ",") + ` ON CONFLICT (document_id, chunk_index) DO NOTHING`

	_, err := s.db.ExecContext(ctx, query, args...)
	return err
}

// GetByID fetches one chunk by its primary key. Returns sql.ErrNoRows if absent.
func (s *ChunksStore) GetByID(ctx context.Context, id string) (*Chunk, error) {
	query := `
SELECT id, document_id, chunk_index, text, token_count, section
FROM chunks WHERE id = $1`

	var c Chunk
	var section sql.NullString
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&c.ID, &c.DocumentID, &c.ChunkIndex, &c.Text, &c.TokenCount, &section,
	)
	if err != nil {
		return nil, err
	}
	if section.Valid {
		c.Section = section.String
	}
	return &c, nil
}

func (s *ChunksStore) GetByDocumentID(ctx context.Context, docID string) ([]Chunk, error) {
	query := `
SELECT id, document_id, chunk_index, text, token_count, section
FROM chunks WHERE document_id = $1 ORDER BY chunk_index`

	rows, err := s.db.QueryContext(ctx, query, docID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chunks []Chunk
	for rows.Next() {
		var c Chunk
		var section sql.NullString
		if err := rows.Scan(&c.ID, &c.DocumentID, &c.ChunkIndex, &c.Text, &c.TokenCount, &section); err != nil {
			return nil, err
		}
		if section.Valid {
			c.Section = section.String
		}
		chunks = append(chunks, c)
	}
	return chunks, rows.Err()
}

// ---- EmbeddingsStore ----

type EmbeddingsStore struct {
	db DBTX
}

func NewEmbeddingsStore(db DBTX) *EmbeddingsStore {
	return &EmbeddingsStore{db: db}
}

// Upsert stores (or replaces) an embedding for a chunk.
// lib/pq has no native []float32 → vector binding, so we format the literal
// ourselves and cast it in SQL with $1::vector.
func (s *EmbeddingsStore) Upsert(ctx context.Context, chunkID string, vec []float32, model string) error {
	query := `
INSERT INTO chunk_embeddings (chunk_id, embedding, model)
VALUES ($1, $2::vector, $3)
ON CONFLICT (chunk_id) DO UPDATE SET
    embedding   = EXCLUDED.embedding,
    model       = EXCLUDED.model,
    embedded_at = CURRENT_TIMESTAMP`

	_, err := s.db.ExecContext(ctx, query, chunkID, vectorLiteral(vec), model)
	return err
}

func (s *EmbeddingsStore) Exists(ctx context.Context, chunkID string) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM chunk_embeddings WHERE chunk_id = $1)`, chunkID,
	).Scan(&exists)
	return exists, err
}

// Search runs a cosine-similarity ANN query using the HNSW index.
// SET (without LOCAL) is connection-scoped. On *sql.DB the pool may assign a
// different connection for the SET and the SELECT, but the setting is harmless
// if it persists on a reused conn — it only affects recall quality, not
// correctness. Using LOCAL would require a transaction, which complicates the
// DBTX interface for minimal benefit.
func (s *EmbeddingsStore) Search(ctx context.Context, vec []float32, opts SearchOpts) ([]ChunkHit, error) {
	// Raise HNSW ef_search for better recall. Advisory: ignored if pgvector
	// doesn't support the GUC (older versions).
	_, _ = s.db.ExecContext(ctx, "SET hnsw.ef_search = 60")

	k := opts.K
	if k <= 0 {
		k = 8
	}

	var symbols interface{}
	if len(opts.Symbols) > 0 {
		symbols = pq.Array(opts.Symbols)
	}
	var sourceTypes interface{}
	if len(opts.SourceTypes) > 0 {
		sourceTypes = pq.Array(opts.SourceTypes)
	}

	query := `
SELECT c.id, c.document_id, c.text, c.section, d.source_url, d.symbol, d.filed_at,
       1 - (e.embedding <=> $1::vector) AS score
FROM chunk_embeddings e
JOIN chunks c ON c.id = e.chunk_id
JOIN documents d ON d.id = c.document_id
WHERE ($2::text[] IS NULL OR d.symbol = ANY($2))
  AND ($3::text[] IS NULL OR d.source_type = ANY($3))
  AND ($4::timestamp IS NULL OR d.filed_at >= $4)
ORDER BY e.embedding <=> $1::vector
LIMIT $5`

	rows, err := s.db.QueryContext(ctx, query,
		vectorLiteral(vec), symbols, sourceTypes, opts.Since, k,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hits []ChunkHit
	for rows.Next() {
		var h ChunkHit
		var section, symbol sql.NullString
		var filedAt sql.NullTime
		if err := rows.Scan(&h.ChunkID, &h.DocumentID, &h.Text, &section,
			&h.SourceURL, &symbol, &filedAt, &h.Score); err != nil {
			return nil, err
		}
		if section.Valid {
			h.Section = section.String
		}
		if symbol.Valid {
			h.Symbol = symbol.String
		}
		if filedAt.Valid {
			h.FiledAt = &filedAt.Time
		}
		hits = append(hits, h)
	}
	return hits, rows.Err()
}

// vectorLiteral formats a []float32 as the pgvector text literal "[0.1,0.2,...]".
// This is necessary because lib/pq has no native binding for the vector type.
func vectorLiteral(vec []float32) string {
	if len(vec) == 0 {
		return "[]"
	}
	sb := strings.Builder{}
	sb.WriteByte('[')
	for i, v := range vec {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(fmt.Sprintf("%g", v))
	}
	sb.WriteByte(']')
	return sb.String()
}

// ---- ResearchQueriesStore ----

type ResearchQueriesStore struct {
	db DBTX
}

func NewResearchQueriesStore(db DBTX) *ResearchQueriesStore {
	return &ResearchQueriesStore{db: db}
}

func (s *ResearchQueriesStore) Insert(ctx context.Context, q ResearchQuery) error {
	citations := q.Citations
	if citations == nil {
		citations = []byte("[]")
	}

	query := `
INSERT INTO research_queries
  (id, user_id, query_text, answer_text, refused, refusal_reason, citations,
   retrieval_ms, generation_ms, total_ms, embed_tokens, prompt_tokens,
   completion_tokens, cost_usd_micros, model)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`

	_, err := s.db.ExecContext(ctx, query,
		q.ID, q.UserID, q.QueryText, q.AnswerText,
		q.Refused, q.RefusalReason, citations,
		q.RetrievalMS, q.GenerationMS, q.TotalMS,
		q.EmbedTokens, q.PromptTokens, q.CompletionTokens,
		q.CostUSDMicros, q.Model,
	)
	return err
}
