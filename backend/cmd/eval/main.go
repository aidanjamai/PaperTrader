// cmd/eval runs the curated golden + refusal sets through AnswerService and
// writes a markdown report. Use to validate citation accuracy, refusal
// behavior, latency, and cost per query before changes ship.
//
// Usage:
//
//	go run ./cmd/eval
//	go run ./cmd/eval -golden cmd/eval/golden.json -out docs/EVAL_RESULTS.md
//	go run ./cmd/eval -skip-cache              # bypass the eval LLM cache
//	go run ./cmd/eval -judge-model llama-3.3-70b-versatile  # default; bigger judge, more reliable
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"

	"papertrader/internal/config"
	"papertrader/internal/data"
	"papertrader/internal/service/research"
)

type goldenItem struct {
	ID                    string   `json:"id"`
	Query                 string   `json:"query"`
	Symbols               []string `json:"symbols"`
	ExpectedSubstringsAny []string `json:"expected_substrings_any"`
	MinCitations          int      `json:"min_citations"`
}

type refusalItem struct {
	ID                    string `json:"id"`
	Query                 string `json:"query"`
	ExpectedRefusalReason string `json:"expected_refusal_reason"`
}

func main() {
	goldenPath := flag.String("golden", "cmd/eval/golden.json", "path to golden.json")
	refusalPath := flag.String("refusal", "cmd/eval/refusal.json", "path to refusal.json")
	outPath := flag.String("out", "docs/EVAL_RESULTS.md", "output markdown path (auto-falls-back to ../docs/... if running from backend/)")
	// Judge defaults to the 70B (same model as generation, but a separate
	// inference). 8B-instant was tried earlier; it was unreliable on
	// near-verbatim chunk-vs-claim pairs. The cross-MODEL story is weaker
	// (same family); the cross-INFERENCE story still holds. Swap to
	// gemini-1.5-flash here when a Gemini key lands.
	judgeModel := flag.String("judge-model", "llama-3.3-70b-versatile", "Groq model for the citation judge")
	skipCache := flag.Bool("skip-cache", false, "bypass the eval LLM cache")
	throttleSec := flag.Int("throttle-seconds", 8, "minimum seconds to wait between Ask calls (helps stay under Groq's 12K TPM)")
	maxRetries := flag.Int("max-retries", 2, "max retries on Groq 429 (token-per-minute) errors per item")
	flag.Parse()

	loaded := false
	for _, path := range []string{".env", "../.env"} {
		if err := godotenv.Load(path); err == nil {
			slog.Info("loaded env file", "path", path)
			loaded = true
			break
		}
	}
	if !loaded {
		slog.Info("no .env file found, using system environment variables")
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "eval: invalid configuration: %v\n", err)
		os.Exit(1)
	}
	if cfg.VoyageAPIKey == "" {
		fmt.Fprintln(os.Stderr, "eval: VOYAGE_API_KEY is required")
		os.Exit(1)
	}
	if cfg.GroqAPIKey == "" {
		fmt.Fprintln(os.Stderr, "eval: GROQ_API_KEY is required")
		os.Exit(1)
	}

	config.SetupLogger(cfg.Environment, cfg.LogLevel)

	db, err := config.ConnectPostgreSQL(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "eval: database connection failed: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	redisClient, err := config.ConnectRedis(cfg)
	if err != nil {
		slog.Warn("Redis unavailable; LLM cache disabled", "err", err)
		redisClient = nil
	}

	voyage := research.NewVoyageEmbedder(cfg.VoyageAPIKey)
	var embedder research.Embedder
	if redisClient != nil {
		embedder = research.NewCachedEmbedder(voyage, redisClient)
	} else {
		embedder = voyage
	}

	genGroq := research.NewGroqClientWithModel(cfg.GroqAPIKey, "llama-3.3-70b-versatile")
	judgeGroq := research.NewGroqClientWithModel(cfg.GroqAPIKey, *judgeModel)

	var genClient research.LLMClient = genGroq
	var judgeClient research.LLMClient = judgeGroq

	if redisClient != nil && !*skipCache {
		genClient = research.NewCachedLLMClient(genGroq, redisClient, "eval:gen:")
		judgeClient = research.NewCachedLLMClient(judgeGroq, redisClient, "eval:judge:")
	}

	embeddingsStore := data.NewEmbeddingsStore(db)
	chunksStore := data.NewChunksStore(db)
	retrieval := research.NewRetrievalService(embedder, embeddingsStore)
	queriesStore := data.NewResearchQueriesStore(db)

	// Pass nil for the answer cache — the eval harness handles caching at the
	// LLM layer; a separate answer cache would suppress the token/cost recording
	// we need for metrics.
	answerSvc := research.NewAnswerService(retrieval, genClient, queriesStore, nil)

	goldenItems, err := loadGolden(*goldenPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "eval: load golden set: %v\n", err)
		os.Exit(1)
	}
	refusalItems, err := loadRefusal(*refusalPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "eval: load refusal set: %v\n", err)
		os.Exit(1)
	}

	total := len(goldenItems) + len(refusalItems)
	ctx := context.Background()

	runStart := time.Now().UTC()

	var goldenResults []ItemResult
	var allLatencies []int
	var totalCostMicros int64
	goldenFalsePositives := 0 // golden items where the system incorrectly refused

	for i, item := range goldenItems {
		if i > 0 && *throttleSec > 0 {
			time.Sleep(time.Duration(*throttleSec) * time.Second)
		}
		fmt.Printf("[%d/%d] %s ...\n", i+1, total, item.ID)

		start := time.Now()
		answer, askErr := askWithRetry(ctx, answerSvc, item.Query, research.AskOpts{
			Symbols: item.Symbols,
		}, *maxRetries)
		if askErr != nil {
			slog.Error("ask failed", "id", item.ID, "err", askErr)
			goldenResults = append(goldenResults, ItemResult{
				ID:      item.ID,
				Query:   item.Query,
				Verdict: "FAIL",
			})
			continue
		}
		latencyMS := int(time.Since(start).Milliseconds())
		allLatencies = append(allLatencies, latencyMS)

		costMicros := fetchCostMicros(ctx, db, answer.QueryID)
		totalCostMicros += costMicros

		if answer.Refused {
			// A refusal on a golden item is a false positive: the system refused
			// a query it should have answered, which inflates precision if uncounted.
			goldenFalsePositives++
			goldenResults = append(goldenResults, ItemResult{
				ID:            item.ID,
				Query:         item.Query,
				Refused:       true,
				RefusalReason: answer.RefusalReason,
				NumCitations:  0,
				LatencyMS:     latencyMS,
				Verdict:       "REFUSED",
			})
			continue
		}

		chunkTexts := fetchChunkTexts(ctx, chunksStore, answer.Citations)
		j := newJudge(embedder, judgeClient, chunkTexts)
		claimScores, judgeErr := j.JudgeAnswer(ctx, answer)
		if judgeErr != nil {
			slog.Warn("judge failed", "id", item.ID, "err", judgeErr)
		}

		var keywordsFound []string
		answerLower := strings.ToLower(answer.Answer)
		for _, kw := range item.ExpectedSubstringsAny {
			if strings.Contains(answerLower, strings.ToLower(kw)) {
				keywordsFound = append(keywordsFound, kw)
			}
		}

		verdict := "FAIL"
		if len(answer.Citations) >= item.MinCitations && len(keywordsFound) > 0 {
			verdict = "PASS"
		}

		goldenResults = append(goldenResults, ItemResult{
			ID:            item.ID,
			Query:         item.Query,
			Refused:       false,
			NumCitations:  len(answer.Citations),
			LatencyMS:     latencyMS,
			ClaimScores:   claimScores,
			KeywordsFound: keywordsFound,
			Verdict:       verdict,
		})
	}

	var refusalResults []ItemResult
	truePositives := 0  // refused AND should have been
	falsePositives := 0 // refused AND should NOT have been (refusal-loop only; golden FPs tracked above)
	falseNegatives := 0 // not refused AND should have been

	for i, item := range refusalItems {
		if (i > 0 || len(goldenItems) > 0) && *throttleSec > 0 {
			time.Sleep(time.Duration(*throttleSec) * time.Second)
		}
		idx := len(goldenItems) + i + 1
		fmt.Printf("[%d/%d] %s ...\n", idx, total, item.ID)

		start := time.Now()
		answer, askErr := askWithRetry(ctx, answerSvc, item.Query, research.AskOpts{}, *maxRetries)
		if askErr != nil {
			slog.Error("ask failed", "id", item.ID, "err", askErr)
			refusalResults = append(refusalResults, ItemResult{
				ID:      item.ID,
				Query:   item.Query,
				Verdict: "FAIL",
			})
			continue
		}
		latencyMS := int(time.Since(start).Milliseconds())
		allLatencies = append(allLatencies, latencyMS)

		costMicros := fetchCostMicros(ctx, db, answer.QueryID)
		totalCostMicros += costMicros

		expectedReason := item.ExpectedRefusalReason
		actualReason := answer.RefusalReason

		verdict := "FAIL"
		if answer.Refused {
			if answer.RefusalReason == expectedReason {
				verdict = "PASS"
				truePositives++
			} else {
				// Refused but wrong reason — still counts as refused for precision.
				falsePositives++
			}
		} else {
			falseNegatives++
		}

		refusalResults = append(refusalResults, ItemResult{
			ID:                  item.ID,
			Query:               item.Query,
			Refused:             answer.Refused,
			RefusalReason:       expectedReason,
			ActualRefusalReason: actualReason,
			LatencyMS:           latencyMS,
			Verdict:             verdict,
		})
	}

	refusalPrecision, refusalRecall := calcRefusalMetrics(goldenFalsePositives, truePositives, falsePositives, falseNegatives)

	citationAccuracy := calcCitationAccuracy(goldenResults)
	p50, p95, p99 := percentiles(allLatencies)

	queryCount := int64(len(goldenItems) + len(refusalItems))
	var meanCostMicros int64
	if queryCount > 0 {
		meanCostMicros = totalCostMicros / queryCount
	}

	metrics := RunMetrics{
		StartedAt:            runStart,
		GoldenSetSize:        len(goldenItems),
		RefusalSetSize:       len(refusalItems),
		CitationAccuracy:     citationAccuracy,
		RefusalPrecision:     refusalPrecision,
		RefusalRecall:        refusalRecall,
		LatencyP50:           p50,
		LatencyP95:           p95,
		LatencyP99:           p99,
		RetrievalLatencyP50:  0, // not tracked per-item; left as 0 for this iteration
		GenerationLatencyP50: 0,
		MeanCostMicros:       meanCostMicros,
		TotalCostMicros:      totalCostMicros,
		GenerationModel:      genClient.Model(),
		JudgeModel:           judgeClient.Model(),
		EmbedderModel:        embedder.Model(),
	}

	resolvedOut := resolveOutputPath(*outPath)
	if err := writeReportWithFallback(resolvedOut, metrics, goldenResults, refusalResults); err != nil {
		fmt.Fprintf(os.Stderr, "eval: %v\n", err)
		os.Exit(1)
	}

	slog.Info("eval complete",
		"citation_accuracy", fmt.Sprintf("%.2f", citationAccuracy),
		"refusal_precision", fmt.Sprintf("%.2f", refusalPrecision),
		"refusal_recall", fmt.Sprintf("%.2f", refusalRecall),
		"out", resolvedOut,
	)
}

// resolveOutputPath handles the common case of running cmd/eval from inside
// backend/, where the default `docs/...` would resolve to a non-existent
// `backend/docs/`. If the parent dir of `out` doesn't exist but `../<out>`
// does, prefer the latter — same logic as the .env fallback in main.
func resolveOutputPath(out string) string {
	if filepath.IsAbs(out) {
		return out
	}
	dir := filepath.Dir(out)
	if _, err := os.Stat(dir); err == nil {
		return out
	}
	alt := filepath.Join("..", out)
	if _, err := os.Stat(filepath.Dir(alt)); err == nil {
		return alt
	}
	return out
}

// writeReportWithFallback creates the file (mkdir-p its parent if missing) and
// writes the report. If create fails, dumps the report to stdout so the run's
// API spend isn't wasted.
func writeReportWithFallback(outPath string, m RunMetrics, golden, refusal []ItemResult) error {
	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		fmt.Fprintf(os.Stderr, "eval: mkdir %s: %v\n", filepath.Dir(outPath), err)
	}
	f, err := os.Create(outPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "eval: create %s: %v\n", outPath, err)
		fmt.Fprintln(os.Stderr, "eval: writing report to stdout as fallback")
		if writeErr := WriteReport(os.Stdout, m, golden, refusal); writeErr != nil {
			return fmt.Errorf("write report to stdout: %w", writeErr)
		}
		return nil
	}
	defer f.Close()
	if err := WriteReport(f, m, golden, refusal); err != nil {
		return fmt.Errorf("write report: %w", err)
	}
	return nil
}

// askWithRetry wraps AnswerService.Ask with a small retry loop for Groq's
// per-minute token limit. Detects the 429 by string match against the wrapped
// error, parses the suggested wait from the message body, and sleeps. Other
// errors are returned immediately. Retries are deliberately bounded.
var groqRetryAfterRe = regexp.MustCompile(`try again in (\d+\.?\d*)(ms|s)`)

func askWithRetry(ctx context.Context, svc *research.AnswerService, query string, opts research.AskOpts, maxRetries int) (*research.Answer, error) {
	attempts := maxRetries + 1
	for i := 0; i < attempts; i++ {
		answer, err := svc.Ask(ctx, "", query, opts)
		if err == nil {
			return answer, nil
		}
		if !strings.Contains(err.Error(), "status 429") || i == attempts-1 {
			return nil, err
		}
		wait := parseGroqRetryAfter(err.Error())
		if wait <= 0 {
			wait = 30 * time.Second
		}
		// Cushion to clear the rolling-window edge.
		wait += 2 * time.Second
		slog.Warn("groq 429 hit; sleeping before retry",
			"wait", wait, "attempt", i+1, "max", attempts)
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return nil, fmt.Errorf("retries exhausted")
}

func parseGroqRetryAfter(msg string) time.Duration {
	m := groqRetryAfterRe.FindStringSubmatch(msg)
	if len(m) != 3 {
		return 0
	}
	n, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0
	}
	if m[2] == "ms" {
		return time.Duration(n) * time.Millisecond
	}
	return time.Duration(n * float64(time.Second))
}

// fetchChunkTexts builds a chunk_id → full text map for the judge. Citation
// excerpts are display-truncated to 250 chars; the judge needs the full chunk
// to fairly evaluate whether the source supports the claim.
func fetchChunkTexts(ctx context.Context, store *data.ChunksStore, citations []research.Citation) map[string]string {
	out := make(map[string]string, len(citations))
	for _, c := range citations {
		if _, ok := out[c.ChunkID]; ok {
			continue
		}
		ch, err := store.GetByID(ctx, c.ChunkID)
		if err != nil {
			slog.Warn("eval: fetch chunk for judge failed; falling back to truncated excerpt",
				"chunk_id", c.ChunkID, "err", err)
			out[c.ChunkID] = c.Excerpt
			continue
		}
		out[c.ChunkID] = ch.Text
	}
	return out
}

// fetchCostMicros queries research_queries for the persisted cost of a single
// query. Option (a): post-hoc lookup rather than extending the Answer struct.
// The AnswerService already persists cost_usd_micros; we just read it back.
func fetchCostMicros(ctx context.Context, db *sql.DB, queryID string) int64 {
	var cost sql.NullInt64
	_ = db.QueryRowContext(ctx,
		`SELECT cost_usd_micros FROM research_queries WHERE id = $1`,
		queryID,
	).Scan(&cost)
	if cost.Valid {
		return cost.Int64
	}
	return 0
}

// calcRefusalMetrics computes precision and recall from raw counters.
// goldenFPs: refusals on golden (should-answer) items — false positives from the golden loop.
// tPs: correct refusals from the refusal loop.
// refusalFPs: refusals with wrong reason from the refusal loop.
// fNs: non-refusals on items that should have been refused.
//
// Precision = truePositives / totalRefusals (across both loops).
// Recall    = truePositives / shouldHaveRefused.
// Both default to 0 when the denominator is zero.
func calcRefusalMetrics(goldenFPs, tPs, refusalFPs, fNs int) (precision, recall float64) {
	refusedTotal := tPs + refusalFPs + goldenFPs
	if refusedTotal > 0 {
		precision = float64(tPs) / float64(refusedTotal)
	}
	shouldRefuse := tPs + fNs
	if shouldRefuse > 0 {
		recall = float64(tPs) / float64(shouldRefuse)
	}
	return
}

func calcCitationAccuracy(results []ItemResult) float64 {
	total, passed := 0, 0
	for _, r := range results {
		for _, cs := range r.ClaimScores {
			total++
			if cs.Pass {
				passed++
			}
		}
	}
	if total == 0 {
		return 0
	}
	return float64(passed) / float64(total)
}

func percentiles(latencies []int) (p50, p95, p99 int) {
	if len(latencies) == 0 {
		return
	}
	sorted := make([]int, len(latencies))
	copy(sorted, latencies)
	sort.Ints(sorted)
	p50 = sorted[int(float64(len(sorted)-1)*0.50)]
	p95 = sorted[int(float64(len(sorted)-1)*0.95)]
	p99 = sorted[int(float64(len(sorted)-1)*0.99)]
	return
}

func loadGolden(path string) ([]goldenItem, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var items []goldenItem
	if err := json.Unmarshal(b, &items); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return items, nil
}

func loadRefusal(path string) ([]refusalItem, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var items []refusalItem
	if err := json.Unmarshal(b, &items); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return items, nil
}

