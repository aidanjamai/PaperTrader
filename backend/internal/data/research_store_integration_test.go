// Package data — integration tests for the research stores.
//
// Run with a real Postgres that has the pgvector extension and the research
// schema applied:
//
//	INTEGRATION_DB_URL=postgres://postgres:postgres@localhost/papertrader?sslmode=disable \
//	  go test -tags=integration ./internal/data/
//
// The test applies the up migration, exercises all stores, then cleans up via
// the down migration. It is safe to run multiple times.

//go:build integration

package data

import (
	"context"
	"database/sql"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

func TestResearchStores_Integration(t *testing.T) {
	dsn := os.Getenv("INTEGRATION_DB_URL")
	if dsn == "" {
		t.Skip("INTEGRATION_DB_URL not set; skipping integration test")
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		t.Fatalf("ping db: %v", err)
	}

	// Enable extension (idempotent; requires pgvector/pgvector image).
	if _, err := db.Exec("CREATE EXTENSION IF NOT EXISTS vector"); err != nil {
		t.Fatalf("create extension: %v", err)
	}

	_, thisFile, _, _ := runtime.Caller(0)
	migrationsDir := filepath.Join(filepath.Dir(thisFile), "../migrations/sql")

	applySQL := func(t *testing.T, file string) {
		t.Helper()
		body, err := os.ReadFile(filepath.Join(migrationsDir, file))
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		if _, err := db.Exec(string(body)); err != nil {
			t.Fatalf("exec %s: %v", file, err)
		}
	}

	// Apply schema; always clean up.
	applySQL(t, "0009_research_schema.up.sql")
	t.Cleanup(func() { applySQL(t, "0009_research_schema.down.sql") })

	ctx := context.Background()

	docStore := NewDocumentsStore(db)
	chunkStore := NewChunksStore(db)
	embStore := NewEmbeddingsStore(db)

	filedAt := time.Date(2024, 10, 30, 0, 0, 0, 0, time.UTC)
	doc := Document{
		ID:         "integ-doc-1",
		SourceType: "sec_filing",
		SourceURL:  "https://sec.gov/integ/test/1",
		Symbol:     "AAPL",
		Title:      "10-K 2024 Integration",
		FiledAt:    &filedAt,
	}
	if err := docStore.Upsert(ctx, doc); err != nil {
		t.Fatalf("Upsert doc: %v", err)
	}

	// Three 1024-dim deterministic embedding patterns: one-hot at indices 0, 100, 200.
	makeVec := func(hotIdx int) []float32 {
		v := make([]float32, 1024)
		v[hotIdx] = 1.0
		return v
	}

	chunks := []Chunk{
		{ID: "integ-chunk-0", DocumentID: "integ-doc-1", ChunkIndex: 0, Text: "Apple supply chain risk factors", TokenCount: 6, Section: "Item 1A"},
		{ID: "integ-chunk-1", DocumentID: "integ-doc-1", ChunkIndex: 1, Text: "Management discussion and analysis", TokenCount: 5, Section: "Item 7"},
		{ID: "integ-chunk-2", DocumentID: "integ-doc-1", ChunkIndex: 2, Text: "Financial statements and notes", TokenCount: 5, Section: "Item 8"},
	}
	if err := chunkStore.BulkInsert(ctx, chunks); err != nil {
		t.Fatalf("BulkInsert: %v", err)
	}

	vecs := [][]float32{makeVec(0), makeVec(100), makeVec(200)}
	model := "voyage-finance-2"
	for i, c := range chunks {
		if err := embStore.Upsert(ctx, c.ID, vecs[i], model); err != nil {
			t.Fatalf("Upsert embedding %d: %v", i, err)
		}
	}

	// Query close to chunk-0 (hot at index 0).
	query := makeVec(0)
	// Nudge slightly to keep score < 1.0 while ensuring chunk-0 wins clearly.
	query[1] = 0.01
	// Normalize.
	var sum float64
	for _, v := range query {
		sum += float64(v) * float64(v)
	}
	norm := float32(math.Sqrt(sum))
	for i := range query {
		query[i] /= norm
	}

	hits, err := embStore.Search(ctx, query, SearchOpts{K: 3})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) == 0 {
		t.Fatal("Search returned no hits")
	}
	if hits[0].ChunkID != "integ-chunk-0" {
		t.Errorf("top hit = %q, want integ-chunk-0", hits[0].ChunkID)
	}
	if hits[0].Score < 0.9 {
		t.Errorf("top hit score = %.4f, want >= 0.9", hits[0].Score)
	}
}
