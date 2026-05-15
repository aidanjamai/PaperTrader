package data

import (
	"context"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func TestDocumentsStore_Upsert(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectExec(`INSERT INTO documents`).
		WithArgs(
			"docid1", "sec_filing", "https://example.com/filing", "AAPL",
			"10-K 2024", sqlmock.AnyArg(), sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	store := NewDocumentsStore(db)
	filedAt := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	doc := Document{
		ID:         "docid1",
		SourceType: "sec_filing",
		SourceURL:  "https://example.com/filing",
		Symbol:     "AAPL",
		Title:      "10-K 2024",
		FiledAt:    &filedAt,
	}
	if err := store.Upsert(context.Background(), doc); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestDocumentsStore_GetByID(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	now := time.Now()
	filedAt := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	rows := sqlmock.NewRows([]string{
		"id", "source_type", "source_url", "symbol", "title", "filed_at", "metadata", "fetched_at",
	}).AddRow("docid1", "sec_filing", "https://example.com/filing", "AAPL", "10-K 2024", filedAt, []byte("{}"), now)

	mock.ExpectQuery(`SELECT .* FROM documents WHERE id`).
		WithArgs("docid1").
		WillReturnRows(rows)

	store := NewDocumentsStore(db)
	doc, err := store.GetByID(context.Background(), "docid1")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if doc == nil {
		t.Fatal("expected document, got nil")
	}
	if doc.Symbol != "AAPL" {
		t.Errorf("symbol = %q, want AAPL", doc.Symbol)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestDocumentsStore_GetByID_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery(`SELECT .* FROM documents WHERE id`).
		WithArgs("missing").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	store := NewDocumentsStore(db)
	doc, err := store.GetByID(context.Background(), "missing")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if doc != nil {
		t.Errorf("expected nil for missing doc, got %+v", doc)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestChunksStore_BulkInsert(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectExec(`INSERT INTO chunks`).
		WithArgs(
			"c1", "doc1", 0, "text one", 10, "",
			"c2", "doc1", 1, "text two", 8, "Item 1A",
		).
		WillReturnResult(sqlmock.NewResult(2, 2))

	store := NewChunksStore(db)
	chunks := []Chunk{
		{ID: "c1", DocumentID: "doc1", ChunkIndex: 0, Text: "text one", TokenCount: 10},
		{ID: "c2", DocumentID: "doc1", ChunkIndex: 1, Text: "text two", TokenCount: 8, Section: "Item 1A"},
	}
	if err := store.BulkInsert(context.Background(), chunks); err != nil {
		t.Fatalf("BulkInsert: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestChunksStore_BulkInsert_Empty(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	store := NewChunksStore(db)
	// No SQL should be issued for an empty slice.
	if err := store.BulkInsert(context.Background(), nil); err != nil {
		t.Fatalf("BulkInsert empty: %v", err)
	}
}

func TestEmbeddingsStore_Upsert_VectorLiteral(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// The vector must arrive as the pgvector text literal format.
	mock.ExpectExec(`INSERT INTO chunk_embeddings`).
		WithArgs("chunk1", "[1,0,0]", "gemini-embedding-001").
		WillReturnResult(sqlmock.NewResult(1, 1))

	store := NewEmbeddingsStore(db)
	vec := []float32{1, 0, 0}
	if err := store.Upsert(context.Background(), "chunk1", vec, "gemini-embedding-001"); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestEmbeddingsStore_Exists(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery(`SELECT EXISTS`).
		WithArgs("chunk1").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

	store := NewEmbeddingsStore(db)
	ok, err := store.Exists(context.Background(), "chunk1")
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if !ok {
		t.Error("expected exists=true")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestEmbeddingsStore_Search(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	filedAt := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	// SET (no LOCAL) is connection-scoped; best-effort, so we allow it.
	mock.ExpectExec(`SET hnsw.ef_search`).WillReturnResult(sqlmock.NewResult(0, 0))

	mock.ExpectQuery(`SELECT c\.id`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "document_id", "text", "section", "source_url", "symbol", "filed_at", "score",
		}).AddRow("c1", "doc1", "some text", "Item 1A", "https://sec.gov/f", "AAPL", filedAt, 0.92))

	store := NewEmbeddingsStore(db)
	vec := []float32{1, 0, 0}
	hits, err := store.Search(context.Background(), vec, SearchOpts{K: 5})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("expected 1 hit, got %d", len(hits))
	}
	if hits[0].Score < 0.9 {
		t.Errorf("score = %.3f, want >= 0.9", hits[0].Score)
	}
	if hits[0].Symbol != "AAPL" {
		t.Errorf("symbol = %q, want AAPL", hits[0].Symbol)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestResearchQueriesStore_Insert(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectExec(`INSERT INTO research_queries`).
		WithArgs(
			"qid1", nil, "What is AAPL revenue?", nil,
			false, nil, sqlmock.AnyArg(),
			nil, nil, nil, nil, nil, nil, nil, nil,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	store := NewResearchQueriesStore(db)
	q := ResearchQuery{
		ID:        "qid1",
		QueryText: "What is AAPL revenue?",
	}
	if err := store.Insert(context.Background(), q); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestVectorLiteral(t *testing.T) {
	cases := []struct {
		in   []float32
		want string
	}{
		{[]float32{}, "[]"},
		{[]float32{1, 0, 0}, "[1,0,0]"},
		{[]float32{0.5, -0.5}, "[0.5,-0.5]"},
	}
	for _, tc := range cases {
		got := vectorLiteral(tc.in)
		if got != tc.want {
			t.Errorf("vectorLiteral(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
