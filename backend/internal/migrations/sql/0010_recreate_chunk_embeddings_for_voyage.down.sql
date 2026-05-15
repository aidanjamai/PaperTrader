-- DESTRUCTIVE: rolling back drops every 1024-dim Voyage embedding and
-- recreates the table at 768 dims (Gemini). There is no recovery path —
-- after rollback, re-run `cmd/ingest` against a Gemini-compatible embedder
-- to repopulate. The chunks table is preserved.

DROP INDEX IF EXISTS idx_chunk_embeddings_vec;
DROP TABLE IF EXISTS chunk_embeddings;

CREATE TABLE chunk_embeddings (
    chunk_id    VARCHAR(64) PRIMARY KEY REFERENCES chunks(id) ON DELETE CASCADE,
    embedding   vector(768) NOT NULL,
    model       VARCHAR(64) NOT NULL,
    embedded_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_chunk_embeddings_vec
  ON chunk_embeddings USING hnsw (embedding vector_cosine_ops);
