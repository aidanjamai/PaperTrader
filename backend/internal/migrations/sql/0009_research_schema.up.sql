CREATE TABLE IF NOT EXISTS documents (
    id              VARCHAR(64) PRIMARY KEY,
    source_type     VARCHAR(20) NOT NULL,
    -- source_url is rendered as a clickable link in the frontend. The CHECK
    -- prevents javascript:/data: URLs from a poisoned ingest source from ever
    -- reaching the DOM; ingest code performs the same check before insert.
    source_url      TEXT NOT NULL CHECK (source_url ~ '^https?://'),
    symbol          VARCHAR(10),
    title           TEXT,
    filed_at        TIMESTAMP,
    metadata        JSONB NOT NULL DEFAULT '{}',
    fetched_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(source_url)
);
CREATE INDEX IF NOT EXISTS idx_documents_symbol_filed_at ON documents(symbol, filed_at DESC);
CREATE INDEX IF NOT EXISTS idx_documents_source_type ON documents(source_type);

CREATE TABLE IF NOT EXISTS chunks (
    id              VARCHAR(64) PRIMARY KEY,
    document_id     VARCHAR(64) NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    chunk_index     INTEGER NOT NULL,
    text            TEXT NOT NULL,
    token_count     INTEGER NOT NULL,
    section         TEXT,
    UNIQUE(document_id, chunk_index)
);
CREATE INDEX IF NOT EXISTS idx_chunks_document_id ON chunks(document_id);

CREATE TABLE IF NOT EXISTS chunk_embeddings (
    chunk_id        VARCHAR(64) PRIMARY KEY REFERENCES chunks(id) ON DELETE CASCADE,
    embedding       vector(768) NOT NULL,
    model           VARCHAR(64) NOT NULL,
    embedded_at     TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_chunk_embeddings_vec
  ON chunk_embeddings USING hnsw (embedding vector_cosine_ops);

CREATE TABLE IF NOT EXISTS research_queries (
    id              VARCHAR(64) PRIMARY KEY,
    user_id         VARCHAR(255) REFERENCES users(id),
    query_text      TEXT NOT NULL,
    answer_text     TEXT,
    refused         BOOLEAN NOT NULL DEFAULT FALSE,
    refusal_reason  VARCHAR(64),
    citations       JSONB NOT NULL DEFAULT '[]',
    retrieval_ms    INTEGER,
    generation_ms   INTEGER,
    total_ms        INTEGER,
    embed_tokens    INTEGER,
    prompt_tokens   INTEGER,
    completion_tokens INTEGER,
    cost_usd_micros INTEGER,
    model           VARCHAR(64),
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_research_queries_user_id ON research_queries(user_id);
CREATE INDEX IF NOT EXISTS idx_research_queries_created_at ON research_queries(created_at DESC);
