package main

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strings"

	"papertrader/internal/service/research"
)

// ClaimScore records the dual-signal judgment for one (claim, citation) pair.
// ChunkExcerpt is the first ~250 chars of the cited chunk's text — surfaced
// in the report so a human can see whether the judge is being unreasonable
// or the generation model is over-claiming.
type ClaimScore struct {
	Claim        string
	CitedChunkID string
	ChunkExcerpt string
	EmbedSim     float64
	LLMJudgeOK   bool
	Pass         bool
}

// Judge scores citation accuracy using embedding similarity and an LLM cross-check.
type Judge struct {
	embedder  research.Embedder
	llmJudge  research.LLMClient
	chunks    map[string]string    // chunk_id → chunk text
	chunkVecs map[string][]float32 // chunk_id → embedding, populated lazily
}

// newJudge takes a chunkID → FULL-TEXT map (not the truncated display excerpt).
// The judge LLM needs the entire chunk to evaluate whether it supports a claim;
// passing only the 250-char Citation.Excerpt caused systematic NO answers
// because the supporting evidence was usually after the truncation point.
func newJudge(embedder research.Embedder, llmJudge research.LLMClient, chunkTexts map[string]string) *Judge {
	return &Judge{
		embedder:  embedder,
		llmJudge:  llmJudge,
		chunks:    chunkTexts,
		chunkVecs: make(map[string][]float32),
	}
}

var citationMarkerRe = regexp.MustCompile(`\[(\d+)\]`)

// JudgeAnswer scores every (claim, citation) pair from an answer.
// Sentences without a citation marker are skipped (counted separately as
// uncited_claims by the caller). Answers with no citations return an empty slice.
func (j *Judge) JudgeAnswer(ctx context.Context, answer *research.Answer) ([]ClaimScore, error) {
	if len(answer.Citations) == 0 {
		return nil, nil
	}

	sentences := splitSentences(answer.Answer)
	var scores []ClaimScore

	for _, sentence := range sentences {
		matches := citationMarkerRe.FindAllStringSubmatch(sentence, -1)
		if len(matches) == 0 {
			continue
		}

		for _, m := range matches {
			n := 0
			fmt.Sscanf(m[1], "%d", &n)
			if n < 1 || n > len(answer.Citations) {
				continue
			}
			cited := answer.Citations[n-1]
			chunkText, ok := j.chunks[cited.ChunkID]
			if !ok {
				continue
			}

			claimVec, _, err := j.embedder.EmbedQuery(ctx, sentence)
			if err != nil {
				return nil, fmt.Errorf("judge: embed claim: %w", err)
			}

			chunkVec, err := j.chunkVec(ctx, cited.ChunkID, chunkText)
			if err != nil {
				return nil, fmt.Errorf("judge: embed chunk: %w", err)
			}

			sim := cosineSim(claimVec, chunkVec)

			judgeResult, err := j.llmJudge.Generate(ctx,
				"You are a strict fact-checker. Reply with exactly YES or NO. Does the SOURCE actually support the CLAIM? Do not consider plausibility — only whether the SOURCE explicitly contains the information needed.",
				fmt.Sprintf("SOURCE: %s\n\nCLAIM: %s", chunkText, sentence),
				research.LLMOpts{Temperature: 0, MaxTokens: 5, JSONMode: false},
			)
			if err != nil {
				return nil, fmt.Errorf("judge: llm call: %w", err)
			}

			llmOK := strings.HasPrefix(strings.TrimSpace(strings.ToUpper(judgeResult.Content)), "YES")

			scores = append(scores, ClaimScore{
				Claim:        sentence,
				CitedChunkID: cited.ChunkID,
				ChunkExcerpt: chunkText,
				EmbedSim:     sim,
				LLMJudgeOK:   llmOK,
				Pass:         sim >= 0.40 && llmOK,
			})
		}
	}

	return scores, nil
}

func (j *Judge) chunkVec(ctx context.Context, chunkID, text string) ([]float32, error) {
	if v, ok := j.chunkVecs[chunkID]; ok {
		return v, nil
	}
	vec, _, err := j.embedder.EmbedQuery(ctx, text)
	if err != nil {
		return nil, err
	}
	j.chunkVecs[chunkID] = vec
	return vec, nil
}

// splitSentences splits on ". ", "! ", "? " and trailing punctuation.
func splitSentences(text string) []string {
	re := regexp.MustCompile(`[.!?]+\s+`)
	parts := re.Split(text, -1)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func cosineSim(a, b []float32) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	var dot, normA, normB float64
	for i := 0; i < n; i++ {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	denom := math.Sqrt(normA) * math.Sqrt(normB)
	if denom == 0 {
		return 0
	}
	return dot / denom
}
