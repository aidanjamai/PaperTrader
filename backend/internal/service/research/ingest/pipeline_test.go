package ingest

import (
	"context"
	"strings"
	"testing"

	"papertrader/internal/data"
	"papertrader/internal/service/research"
)

// --- stubs ---

type stubFetcher struct {
	filings     []Filing
	fetchCalled map[string]int // url → call count
	textByURL   map[string]string
}

func newStubFetcher(filings []Filing, textByURL map[string]string) *stubFetcher {
	return &stubFetcher{filings: filings, fetchCalled: map[string]int{}, textByURL: textByURL}
}

func (s *stubFetcher) ResolveCIK(_ context.Context, _ string) (string, error) {
	return "0000000001", nil
}

func (s *stubFetcher) FetchRecentFilings(_ context.Context, _ string, _ []string, _ int) ([]Filing, error) {
	return s.filings, nil
}

func (s *stubFetcher) FetchFilingText(_ context.Context, f Filing) (string, error) {
	s.fetchCalled[f.URL]++
	if t, ok := s.textByURL[f.URL]; ok {
		return t, nil
	}
	return strings.Repeat("word ", 200), nil
}

type stubDocStore struct {
	existing map[string]*data.Document // id → doc (nil means not found)
	upserted []data.Document
}

func newStubDocStore(existing map[string]*data.Document) *stubDocStore {
	return &stubDocStore{existing: existing}
}

func (s *stubDocStore) GetByID(_ context.Context, id string) (*data.Document, error) {
	if doc, ok := s.existing[id]; ok {
		return doc, nil
	}
	return nil, nil
}

func (s *stubDocStore) Upsert(_ context.Context, doc data.Document) error {
	s.upserted = append(s.upserted, doc)
	return nil
}

type stubChunkStore struct {
	inserted    int
	chunksByDoc map[string][]data.Chunk // docID → chunks (for GetByDocumentID)
}

func (s *stubChunkStore) BulkInsert(_ context.Context, chunks []data.Chunk) error {
	s.inserted += len(chunks)
	return nil
}

func (s *stubChunkStore) GetByDocumentID(_ context.Context, docID string) ([]data.Chunk, error) {
	return s.chunksByDoc[docID], nil
}

type stubEmbeddingStore struct {
	upserted    int
	embedCalled []string       // chunk IDs passed to Upsert, in order
	existing    map[string]bool // chunk IDs that already have embeddings
}

func (s *stubEmbeddingStore) Upsert(_ context.Context, chunkID string, _ []float32, _ string) error {
	s.upserted++
	s.embedCalled = append(s.embedCalled, chunkID)
	return nil
}

func (s *stubEmbeddingStore) Exists(_ context.Context, chunkID string) (bool, error) {
	return s.existing[chunkID], nil
}

type stubBatchEmbedder struct {
	batchCallCount int
	inputsSeen     []string
}

func (s *stubBatchEmbedder) EmbedQuery(_ context.Context, _ string) ([]float32, int, error) {
	return []float32{0.1, 0.2}, 10, nil
}

func (s *stubBatchEmbedder) EmbedBatch(_ context.Context, texts []string) ([][]float32, int, error) {
	s.batchCallCount++
	s.inputsSeen = append(s.inputsSeen, texts...)
	out := make([][]float32, len(texts))
	for i := range texts {
		out[i] = []float32{0.1, 0.2}
	}
	return out, len(texts) * 10, nil
}

func (s *stubBatchEmbedder) Model() string { return "stub" }

var _ research.Embedder = (*stubBatchEmbedder)(nil)

// buildPipeline wires up a Pipeline from the given stubs.
func buildPipeline(
	fetcher filingFetcher,
	docs documentStore,
	chunks chunkStore,
	embeddings embeddingStore,
	embedder research.Embedder,
) *Pipeline {
	return &Pipeline{
		edgar:      fetcher,
		embedder:   embedder,
		docs:       docs,
		chunks:     chunks,
		embeddings: embeddings,
	}
}

// --- tests ---

// TestPipeline_Idempotency checks that a filing whose doc ID already exists
// and all chunks are embedded is skipped: FetchFilingText is NOT called and
// Skipped is incremented.
func TestPipeline_Idempotency(t *testing.T) {
	filingURL := "https://www.sec.gov/Archives/edgar/data/1/000000000124000001/doc.htm"
	existingURL := "https://www.sec.gov/Archives/edgar/data/1/000000000124000002/old.htm"

	newFiling := Filing{URL: filingURL, FormType: "10-K", FiledAt: "2024-01-01", CIK: "0000000001", AccessionNumber: "0000000001-24-000001"}
	oldFiling := Filing{URL: existingURL, FormType: "10-K", FiledAt: "2023-01-01", CIK: "0000000001", AccessionNumber: "0000000001-23-000002"}

	existingDocID := DocIDFromURL(existingURL)
	existingDoc := &data.Document{ID: existingDocID, SourceURL: existingURL}

	// The old filing already has one chunk with an embedding.
	existingChunkID := ChunkIDFromParts(existingDocID, 0)
	chunkSt := &stubChunkStore{
		chunksByDoc: map[string][]data.Chunk{
			existingDocID: {{ID: existingChunkID, DocumentID: existingDocID, ChunkIndex: 0, Text: "old text", TokenCount: 2}},
		},
	}
	embSt := &stubEmbeddingStore{existing: map[string]bool{existingChunkID: true}}

	fetcher := newStubFetcher([]Filing{oldFiling, newFiling}, nil)
	docs := newStubDocStore(map[string]*data.Document{existingDocID: existingDoc})

	p := buildPipeline(fetcher, docs, chunkSt, embSt, &stubBatchEmbedder{})
	result, err := p.IngestSymbol(context.Background(), "TEST", IngestOpts{FormTypes: []string{"10-K"}, MaxFilings: 5})
	if err != nil {
		t.Fatal(err)
	}

	if result.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1", result.Skipped)
	}
	if fetcher.fetchCalled[existingURL] != 0 {
		t.Errorf("FetchFilingText called for existing filing %q; should have been skipped", existingURL)
	}
	if result.DocumentsAdded != 1 {
		t.Errorf("DocumentsAdded = %d, want 1 (the new filing)", result.DocumentsAdded)
	}
}

// TestPipeline_HappyPath verifies that a new filing is fully processed:
// text fetched, chunks inserted, embeddings upserted, counts match.
func TestPipeline_HappyPath(t *testing.T) {
	// ~200 words × 5 chars avg = ~250 tokens → ChunkText(500) should produce 1 chunk.
	filingText := strings.Repeat("word ", 200)
	filingURL := "https://www.sec.gov/Archives/edgar/data/1/000000000124000001/doc.htm"
	filing := Filing{URL: filingURL, FormType: "10-K", FiledAt: "2024-01-01", CIK: "0000000001", AccessionNumber: "0000000001-24-000001"}

	fetcher := newStubFetcher([]Filing{filing}, map[string]string{filingURL: filingText})
	docs := newStubDocStore(nil)
	chunkSt := &stubChunkStore{}
	embSt := &stubEmbeddingStore{existing: map[string]bool{}}

	p := buildPipeline(fetcher, docs, chunkSt, embSt, &stubBatchEmbedder{})
	result, err := p.IngestSymbol(context.Background(), "TEST", IngestOpts{FormTypes: []string{"10-K"}, MaxFilings: 5})
	if err != nil {
		t.Fatal(err)
	}

	if result.DocumentsAdded != 1 {
		t.Errorf("DocumentsAdded = %d, want 1", result.DocumentsAdded)
	}
	if result.ChunksAdded == 0 {
		t.Error("ChunksAdded = 0, want > 0")
	}
	if result.EmbeddingsAdded != result.ChunksAdded {
		t.Errorf("EmbeddingsAdded = %d, want %d (one per chunk)", result.EmbeddingsAdded, result.ChunksAdded)
	}
	if chunkSt.inserted != result.ChunksAdded {
		t.Errorf("stubChunkStore.inserted = %d, want %d", chunkSt.inserted, result.ChunksAdded)
	}
	if embSt.upserted != result.EmbeddingsAdded {
		t.Errorf("stubEmbeddingStore.upserted = %d, want %d", embSt.upserted, result.EmbeddingsAdded)
	}
	if result.Skipped != 0 {
		t.Errorf("Skipped = %d, want 0", result.Skipped)
	}
}

// TestPipeline_BackfillMissingEmbeddings simulates a partial-embedding failure:
// the document exists, 5 chunks exist in the DB, 2 already have embeddings, 3
// don't. After IngestSymbol (force=false) the embedder must be called for exactly
// 3 chunks and EmbeddingsAdded must equal 3. FetchFilingText must NOT be called.
func TestPipeline_BackfillMissingEmbeddings(t *testing.T) {
	filingURL := "https://www.sec.gov/Archives/edgar/data/1/000000000124000001/partial.htm"
	filing := Filing{URL: filingURL, FormType: "10-K", FiledAt: "2024-01-01", CIK: "0000000001", AccessionNumber: "0000000001-24-000001"}

	docID := DocIDFromURL(filingURL)
	existingDoc := &data.Document{ID: docID, SourceURL: filingURL}

	// Build 5 chunks; indices 0 and 1 already have embeddings.
	var dbChunks []data.Chunk
	embeddedIDs := map[string]bool{}
	for i := 0; i < 5; i++ {
		cid := ChunkIDFromParts(docID, i)
		dbChunks = append(dbChunks, data.Chunk{
			ID:         cid,
			DocumentID: docID,
			ChunkIndex: i,
			Text:       strings.Repeat("word ", 20),
			TokenCount: 20,
		})
		if i < 2 {
			embeddedIDs[cid] = true
		}
	}

	fetcher := newStubFetcher([]Filing{filing}, nil)
	docs := newStubDocStore(map[string]*data.Document{docID: existingDoc})
	chunkSt := &stubChunkStore{
		chunksByDoc: map[string][]data.Chunk{docID: dbChunks},
	}
	embSt := &stubEmbeddingStore{existing: embeddedIDs}
	embedder := &stubBatchEmbedder{}

	p := buildPipeline(fetcher, docs, chunkSt, embSt, embedder)
	result, err := p.IngestSymbol(context.Background(), "TEST", IngestOpts{
		FormTypes:  []string{"10-K"},
		MaxFilings: 5,
		Force:      false,
	})
	if err != nil {
		t.Fatal(err)
	}

	if fetcher.fetchCalled[filingURL] != 0 {
		t.Errorf("FetchFilingText called %d time(s); want 0 (doc already exists)", fetcher.fetchCalled[filingURL])
	}
	if result.EmbeddingsAdded != 3 {
		t.Errorf("EmbeddingsAdded = %d, want 3", result.EmbeddingsAdded)
	}
	if embSt.upserted != 3 {
		t.Errorf("embeddings upserted = %d, want 3", embSt.upserted)
	}
	if len(embedder.inputsSeen) != 3 {
		t.Errorf("embedder received %d texts, want 3", len(embedder.inputsSeen))
	}
	if result.Skipped != 0 {
		t.Errorf("Skipped = %d, want 0 (partial fill should not count as skipped)", result.Skipped)
	}
}
