package ingest

import (
	"strings"
	"testing"
)

const fixtureText = `Item 1A. Risk Factors

Apple faces many risks in its supply chain operations.
Concentration of manufacturing in a single region poses challenges.
Disruptions to logistics may impact revenue significantly.

Item 7. Management Discussion and Analysis

The company reported strong revenue growth in fiscal year 2024.
Operating margins expanded due to product mix improvement.
Services revenue reached a new all-time high.

The following section discusses results in detail.
Revenue increased year-over-year driven by iPhone sales.
Mac revenue was flat compared to the prior year.
iPad category declined modestly due to product cycle timing.`

func TestChunkText_Count(t *testing.T) {
	opts := ChunkOpts{TargetTokens: 50, OverlapTokens: 10}
	chunks := ChunkText(fixtureText, opts)
	if len(chunks) == 0 {
		t.Fatal("expected at least one chunk")
	}
	// Each chunk should be under TargetTokens + a small buffer (overlap can push slightly over).
	for _, c := range chunks {
		// Allow up to 2× TargetTokens because a single sentence can exceed target.
		if c.TokenCount > opts.TargetTokens*2 {
			t.Errorf("chunk %d has %d tokens, expected <= %d", c.Index, c.TokenCount, opts.TargetTokens*2)
		}
	}
}

func TestChunkText_OverlapPresent(t *testing.T) {
	opts := ChunkOpts{TargetTokens: 60, OverlapTokens: 15}
	chunks := ChunkText(fixtureText, opts)
	if len(chunks) < 2 {
		t.Skip("not enough chunks to verify overlap")
	}
	// The start of chunk[1] should contain some words from the end of chunk[0].
	lastWordsChunk0 := strings.Fields(chunks[0].Text)
	if len(lastWordsChunk0) == 0 {
		t.Fatal("chunk 0 is empty")
	}
	lastWord := lastWordsChunk0[len(lastWordsChunk0)-1]
	if !strings.Contains(chunks[1].Text, lastWord) {
		// Overlap is word-based so a word from the end of chunk[0] must appear in chunk[1].
		// This is a soft check — overlap is approximate.
		t.Errorf("overlap check: last word %q not found in chunk[1] (overlap broken)", lastWord)
	}
}

func TestChunkText_SectionAttached(t *testing.T) {
	opts := ChunkOpts{TargetTokens: 50, OverlapTokens: 5}
	chunks := ChunkText(fixtureText, opts)

	foundRisk := false
	foundMDA := false
	for _, c := range chunks {
		if strings.Contains(c.Section, "1A") {
			foundRisk = true
		}
		if strings.Contains(c.Section, "7") {
			foundMDA = true
		}
	}
	if !foundRisk {
		t.Error("expected at least one chunk with section containing '1A'")
	}
	if !foundMDA {
		t.Error("expected at least one chunk with section containing '7'")
	}
}

func TestChunkText_IndexMonotonic(t *testing.T) {
	opts := ChunkOpts{TargetTokens: 40, OverlapTokens: 10}
	chunks := ChunkText(fixtureText, opts)
	for i, c := range chunks {
		if c.Index != i {
			t.Errorf("chunk index mismatch: got %d, want %d", c.Index, i)
		}
	}
}

func TestChunkText_EmptyInput(t *testing.T) {
	chunks := ChunkText("", ChunkOpts{TargetTokens: 500, OverlapTokens: 80})
	if len(chunks) != 0 {
		t.Errorf("expected 0 chunks for empty input, got %d", len(chunks))
	}
}
