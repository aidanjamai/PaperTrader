package ingest

import (
	"regexp"
	"strings"
)

// Chunk is a text fragment produced by ChunkText.
type Chunk struct {
	Index      int
	Text       string
	TokenCount int
	Section    string
}

// ChunkOpts controls the chunking strategy.
type ChunkOpts struct {
	TargetTokens  int
	OverlapTokens int
}

// approxTokens estimates the token count of s using the heuristic len/4,
// which matches English text closely enough for budgeting purposes.
func approxTokens(s string) int {
	return len(s) / 4
}

// sectionHeaderRe matches common 10-K section headers such as
// "Item 1A.", "Item 7.", "Item 15.", "ITEM 1A.", etc.
var sectionHeaderRe = regexp.MustCompile(`(?i)^\s*(Item\s+\d+[A-Za-z]?\.?)`)

// ChunkText splits text into overlapping chunks of approximately TargetTokens.
// Strategy:
//  1. Split on paragraph boundaries ("\n\n").
//  2. If a paragraph exceeds TargetTokens, fall back to sentence splitting (". ").
//  3. Greedy-pack segments into chunks up to TargetTokens.
//  4. Carry OverlapTokens of trailing text into the next chunk.
//  5. Track the most-recently-seen section header and attach it to chunks.
func ChunkText(text string, opts ChunkOpts) []Chunk {
	if opts.TargetTokens <= 0 {
		opts.TargetTokens = 500
	}
	if opts.OverlapTokens <= 0 {
		opts.OverlapTokens = 80
	}

	paragraphs := strings.Split(text, "\n\n")
	// Flatten into segments ≤ TargetTokens each.
	var segments []string
	for _, para := range paragraphs {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}
		if approxTokens(para) <= opts.TargetTokens {
			segments = append(segments, para)
		} else {
			// Fall back to sentence split.
			sentences := strings.SplitAfter(para, ". ")
			for _, s := range sentences {
				s = strings.TrimSpace(s)
				if s != "" {
					segments = append(segments, s)
				}
			}
		}
	}

	var chunks []Chunk
	var current strings.Builder
	currentTokens := 0
	// chunkSection tracks the section that was in force when the current chunk
	// started accumulating. It is snapshotted at the start of each chunk so
	// that a section header encountered mid-accumulation doesn't retroactively
	// relabel the already-buffered text.
	chunkSection := ""
	// latestSection is the most recently seen section header across all segments.
	latestSection := ""
	chunkIndex := 0
	var overlapTail string // text carried from the previous chunk

	flush := func() {
		text := strings.TrimSpace(current.String())
		if text == "" {
			return
		}
		chunks = append(chunks, Chunk{
			Index:      chunkIndex,
			Text:       text,
			TokenCount: approxTokens(text),
			Section:    chunkSection,
		})
		chunkIndex++

		// Compute overlap: take the last OverlapTokens worth of text from the end.
		words := strings.Fields(text)
		// Each word ≈ 1 token for this heuristic (close enough).
		overlap := 0
		tail := []string{}
		for i := len(words) - 1; i >= 0 && overlap < opts.OverlapTokens; i-- {
			overlap += approxTokens(words[i])
			tail = append([]string{words[i]}, tail...)
		}
		overlapTail = strings.Join(tail, " ")

		current.Reset()
		currentTokens = 0
		// The new chunk inherits the current section context.
		chunkSection = latestSection
		if overlapTail != "" {
			current.WriteString(overlapTail)
			current.WriteByte('\n')
			currentTokens = approxTokens(overlapTail)
		}
	}

	for _, seg := range segments {
		segTokens := approxTokens(seg)

		// Flush before appending if this segment would overflow the target.
		if currentTokens+segTokens > opts.TargetTokens && currentTokens > 0 {
			flush()
		}

		// Detect a section header in this segment. We update latestSection
		// before writing to current so that if this segment opens a new chunk
		// (after flush above), chunkSection is already correct.
		if m := sectionHeaderRe.FindString(seg); m != "" {
			latestSection = strings.TrimSpace(m)
			// If the current buffer is empty (just started or just flushed),
			// update chunkSection immediately so this chunk carries the header.
			if current.Len() == 0 || strings.TrimSpace(current.String()) == "" {
				chunkSection = latestSection
			}
		}

		if current.Len() > 0 {
			current.WriteByte('\n')
		}
		current.WriteString(seg)
		currentTokens += segTokens
	}
	flush()
	return chunks
}
