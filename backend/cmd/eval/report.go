package main

import (
	"fmt"
	"io"
	"strings"
	"time"
)

// RunMetrics holds aggregate statistics for one eval run.
type RunMetrics struct {
	StartedAt             time.Time
	GoldenSetSize         int
	RefusalSetSize        int
	CitationAccuracy      float64
	RefusalPrecision      float64
	RefusalRecall         float64
	LatencyP50            int
	LatencyP95            int
	LatencyP99            int
	RetrievalLatencyP50   int
	GenerationLatencyP50  int
	MeanCostMicros        int64
	TotalCostMicros       int64
	GenerationModel       string
	JudgeModel            string
	EmbedderModel         string
}

// ItemResult records the outcome of one golden or refusal item.
// For refusal items, RefusalReason holds the *expected* reason and
// ActualRefusalReason holds what the system actually returned (empty if
// the system did not refuse at all). Splitting them lets the report show
// "Expected" vs "Got" honestly even when they mismatch.
type ItemResult struct {
	ID                  string
	Query               string
	Refused             bool
	RefusalReason       string
	ActualRefusalReason string
	NumCitations        int
	LatencyMS           int
	ClaimScores         []ClaimScore
	KeywordsFound       []string
	Verdict             string // "PASS" | "FAIL" | "REFUSED"
}

// WriteReport writes a deterministic markdown report to out.
func WriteReport(out io.Writer, metrics RunMetrics, golden []ItemResult, refusal []ItemResult) error {
	w := &errWriter{w: out}

	w.printf("# Eval Run — %s\n\n", metrics.StartedAt.UTC().Format("2006-01-02"))

	w.printf("| Metric | Value |\n")
	w.printf("|---|---|\n")
	w.printf("| Generation model | %s |\n", metrics.GenerationModel)
	w.printf("| Judge model | %s |\n", metrics.JudgeModel)
	w.printf("| Embedder | %s |\n", metrics.EmbedderModel)
	w.printf("| Golden set size | %d |\n", metrics.GoldenSetSize)
	w.printf("| Refusal set size | %d |\n", metrics.RefusalSetSize)
	w.printf("| Citation accuracy | %.2f |\n", metrics.CitationAccuracy)
	w.printf("| Refusal precision | %.2f |\n", metrics.RefusalPrecision)
	w.printf("| Refusal recall | %.2f |\n", metrics.RefusalRecall)
	w.printf("| p50 latency | %d ms |\n", metrics.LatencyP50)
	w.printf("| p95 latency | %d ms |\n", metrics.LatencyP95)
	w.printf("| p99 latency | %d ms |\n", metrics.LatencyP99)
	w.printf("| p50 retrieval | %d ms |\n", metrics.RetrievalLatencyP50)
	w.printf("| p50 generation | %d ms |\n", metrics.GenerationLatencyP50)
	w.printf("| Mean cost / query | $%.6f |\n", float64(metrics.MeanCostMicros)/1_000_000)
	w.printf("| Total run cost | $%.6f |\n", float64(metrics.TotalCostMicros)/1_000_000)

	// Per-item wall-clock latency is omitted from individual blocks because it
	// varies with Redis/DB network jitter even on full cache hits, making the
	// report non-deterministic. Latency is aggregated in the header table above.
	w.printf("\n## Golden Set\n")
	for _, item := range golden {
		w.printf("\n### %s — %s\n", item.ID, item.Verdict)
		w.printf("**Query:** %s\n", item.Query)
		w.printf("**Citations:** %d\n", item.NumCitations)
		if len(item.KeywordsFound) > 0 {
			w.printf("**Keywords matched:** %s\n", strings.Join(item.KeywordsFound, ", "))
		} else {
			w.printf("**Keywords matched:** none\n")
		}
		if len(item.ClaimScores) > 0 {
			w.printf("**Claim scores:**\n")
			for i, cs := range item.ClaimScores {
				passStr := "FAIL"
				if cs.Pass {
					passStr = "PASS"
				}
				judgeStr := "NO"
				if cs.LLMJudgeOK {
					judgeStr = "YES"
				}
				chunkShort := cs.CitedChunkID
				if len(chunkShort) > 8 {
					chunkShort = chunkShort[:8] + "…"
				}
				w.printf("- claim %d → chunk %s → embed=%.2f judge=%s **%s**\n",
					i+1, chunkShort, cs.EmbedSim, judgeStr, passStr)
				if cs.Claim != "" {
					w.printf("  - Q: %s\n", oneline(cs.Claim))
				}
				if cs.ChunkExcerpt != "" {
					w.printf("  - A: %s\n", oneline(truncate(cs.ChunkExcerpt, 280)))
				}
			}
		}
	}

	w.printf("\n## Refusal Set\n")
	for _, item := range refusal {
		label := "PASS (refused as expected)"
		if item.Verdict == "FAIL" {
			label = "FAIL"
		}
		w.printf("\n### %s — %s\n", item.ID, label)
		w.printf("**Query:** %s\n", item.Query)
		w.printf("**Expected reason:** %s\n", item.RefusalReason)
		got := item.ActualRefusalReason
		if !item.Refused {
			got = "(not refused — LLM gave a real answer)"
		} else if got == "" {
			got = "(refused, reason unknown)"
		}
		w.printf("**Got:** %s\n", got)
	}

	return w.err
}

// errWriter accumulates the first write error so callers don't need to check
// after every printf.
type errWriter struct {
	w   io.Writer
	err error
}

func (ew *errWriter) printf(format string, args ...any) {
	if ew.err != nil {
		return
	}
	_, ew.err = fmt.Fprintf(ew.w, format, args...)
}

// oneline collapses internal whitespace runs to single spaces so a multi-line
// chunk excerpt or wrapped claim sentence renders cleanly inside a markdown
// list item.
func oneline(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// truncate cuts s to at most n characters, appending an ellipsis when cut.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
