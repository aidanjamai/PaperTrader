-- DESTRUCTIVE: drops and recreates chunk_embeddings to widen the vector
-- column from 768 dims (Gemini) to 1024 dims (Voyage finance-2). pgvector
-- does not allow ALTER COLUMN TYPE on vector(N), so a wipe-and-recreate is
-- the only path.
--
-- Effect: every row in chunk_embeddings is lost. The chunks table is
-- preserved so the ingest pipeline's backfill path (Pipeline.ingestFiling
-- detects missing embeddings via embeddings.Exists and re-embeds each
-- chunk) restores embeddings on the next `cmd/ingest` run. Operators MUST
-- re-run ingest after this migration applies, otherwise retrieval returns
-- nothing.
--
-- If this migration ever runs against a database that already has 1024-dim
-- rows, those rows are also lost. Treat re-running as a destructive op.

DROP INDEX IF EXISTS idx_chunk_embeddings_vec;
DROP TABLE IF EXISTS chunk_embeddings;

CREATE TABLE chunk_embeddings (
    chunk_id    VARCHAR(64) PRIMARY KEY REFERENCES chunks(id) ON DELETE CASCADE,
    embedding   vector(1024) NOT NULL,
    model       VARCHAR(64) NOT NULL,
    embedded_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_chunk_embeddings_vec
  ON chunk_embeddings USING hnsw (embedding vector_cosine_ops);
