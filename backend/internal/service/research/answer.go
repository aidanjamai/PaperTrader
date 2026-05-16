package research

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"papertrader/internal/data"

	"github.com/redis/go-redis/v9"
)

const (
	answerCacheTTL = time.Hour
	maxExcerptLen  = 250
)

var (
	forwardLookingRe = regexp.MustCompile(
		`(?i)\b(should|will|would|going to)\b.+\b(buy|sell|hold|invest|outperform|beat|crash|moon|skyrocket)\b`,
	)
	forecastTermRe = regexp.MustCompile(
		`(?i)\b(predict|forecast|next week|next month|next quarter|target price)\b`,
	)
	// citationMarkerRe captures the N inside [N] markers in the LLM's answer.
	// 1-3 digits is enough — we cap retrieval at K=8 by default, and even
	// generous retrieval bounded by the schema will not exceed 3 digits.
	citationMarkerRe = regexp.MustCompile(`\[(\d{1,3})\]`)
)

// retriever is the subset of RetrievalService used by AnswerService.
// Keeping this an interface lets tests inject a stub without a DB.
type retriever interface {
	Retrieve(ctx context.Context, query string, opts RetrieveOpts) ([]data.ChunkHit, error)
}

// queriesStore is the subset of data.ResearchQueriesStore used by AnswerService.
type queriesStore interface {
	Insert(ctx context.Context, q data.ResearchQuery) error
}

// docSymbolSource is the subset of data.DocumentsStore used to surface corpus
// coverage in no_sources refusals. An interface keeps AnswerService testable.
type docSymbolSource interface {
	DistinctSymbols(ctx context.Context) ([]string, error)
}

const coverageTTL = time.Minute

// coverageCache memoises the distinct-symbol list for a short window so the
// refusal path doesn't hit the DB on every miss. A best-effort hint: on error
// it serves whatever it last had (possibly nil) rather than failing the request.
type coverageCache struct {
	src docSymbolSource

	mu      sync.Mutex
	symbols []string
	fetched time.Time
}

func (c *coverageCache) get(ctx context.Context) []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.symbols != nil && time.Since(c.fetched) < coverageTTL {
		return c.symbols
	}
	syms, err := c.src.DistinctSymbols(ctx)
	if err != nil {
		return c.symbols
	}
	c.symbols = syms
	c.fetched = time.Now()
	return syms
}

// AnswerService orchestrates retrieval, LLM generation, and citation validation.
type AnswerService struct {
	retrieval   retriever
	llm         LLMClient
	queries     queriesStore
	answerCache *redis.Client // optional, nil-safe
	coverage    *coverageCache // optional, nil-safe
	// fallbackGeneral mirrors RESEARCH_FALLBACK_GENERAL. When false, an
	// AllowGeneral request is treated exactly like a normal request (no
	// uncited path), so the feature is fully off by default.
	fallbackGeneral bool
}

func NewAnswerService(
	ret retriever,
	llm LLMClient,
	queries queriesStore,
	cache *redis.Client,
	docSymbols docSymbolSource,
	fallbackGeneral bool,
) *AnswerService {
	s := &AnswerService{
		retrieval:       ret,
		llm:             llm,
		queries:         queries,
		answerCache:     cache,
		fallbackGeneral: fallbackGeneral,
	}
	if docSymbols != nil {
		s.coverage = &coverageCache{src: docSymbols}
	}
	return s
}

// Citation is one source that the LLM cited in its answer.
type Citation struct {
	ChunkID   string  `json:"chunk_id"`
	SourceURL string  `json:"source_url"`
	Symbol    string  `json:"symbol,omitempty"`
	FiledAt   string  `json:"filed_at,omitempty"`
	Excerpt   string  `json:"excerpt"`
	Score     float64 `json:"score"`
}

// Coverage tells the client which tickers the corpus can actually answer for.
// Populated only on no_sources refusals so the UI can offer actionable
// suggestions instead of a dead end.
type Coverage struct {
	Symbols  []string `json:"symbols"`
	Examples []string `json:"examples,omitempty"`
}

// Answer is the top-level response type returned from Ask.
type Answer struct {
	QueryID       string     `json:"query_id"`
	Answer        string     `json:"answer"`
	Citations     []Citation `json:"citations"`
	Refused       bool       `json:"refused"`
	RefusalReason string     `json:"refusal_reason,omitempty"`
	Coverage      *Coverage  `json:"coverage,omitempty"`
	// Mode is "general" for an explicitly opted-in, uncited general-knowledge
	// answer; empty/absent means a normal grounded answer. The client uses
	// this to render an unmistakable "not from filings" banner.
	Mode      string `json:"mode,omitempty"`
	LatencyMS int    `json:"latency_ms"`
}

// AskOpts parameterises a single Ask call.
type AskOpts struct {
	Symbols  []string
	K        int
	MinScore float64
	// AllowGeneral is set only when the user explicitly opts into an uncited
	// general-knowledge answer after a no_sources refusal. Gated server-side
	// by RESEARCH_FALLBACK_GENERAL; never enables advice (the forward-looking
	// gate still runs first).
	AllowGeneral bool
}

func (s *AnswerService) Ask(ctx context.Context, userID, query string, opts AskOpts) (*Answer, error) {
	start := time.Now()

	k := opts.K
	if k <= 0 {
		k = 8
	}
	minScore := opts.MinScore
	if minScore <= 0 {
		// Tuned for voyage-finance-2: genuine matches on financial text
		// cluster in 0.40-0.50, not 0.55+. Higher floors over-refuse.
		minScore = 0.40
	}

	queryID := makeQueryID(query, userID, start)

	// 1. Answer cache lookup. Skipped for AllowGeneral requests: the cache key
	// does not encode the general/grounded distinction, so serving a cached
	// grounded answer to a general request (or vice-versa) would be wrong.
	cacheKey := answerCacheKey(query, opts.Symbols)
	if s.answerCache != nil && !opts.AllowGeneral {
		if cached, err := s.answerCache.Get(ctx, cacheKey).Bytes(); err == nil {
			var a Answer
			if jsonErr := json.Unmarshal(cached, &a); jsonErr == nil {
				a.QueryID = queryID
				return &a, nil
			}
		}
	}

	// 2. Forward-looking refusal (no retrieval, no LLM). MUST stay ahead of the
	// general-knowledge path below so an opted-in uncited answer can never
	// become buy/sell/prediction advice.
	if isForwardLooking(query) {
		return s.refuse(ctx, queryID, userID, query, "forward_looking", start, nil)
	}

	// 2b. Explicit, user-opted uncited general-knowledge answer. Gated by
	// RESEARCH_FALLBACK_GENERAL server-side. Skips retrieval entirely (the
	// user already saw a no_sources refusal and chose to proceed without
	// sources); cheaper and avoids the no_sources/INSUFFICIENT_SOURCES
	// ambiguity. Not cached.
	if opts.AllowGeneral && s.fallbackGeneral {
		return s.generalAnswer(ctx, queryID, userID, query, start)
	}

	// 3. Retrieve.
	retrieveStart := time.Now()
	hits, err := s.retrieval.Retrieve(ctx, query, RetrieveOpts{
		Symbols:  opts.Symbols,
		K:        k,
		MinScore: minScore,
	})
	if err != nil {
		slog.Error("research retrieval failed", "query_id", queryID, "err", err)
		return nil, err
	}
	retrievalMS := int(time.Since(retrieveStart).Milliseconds())

	// 4. No sources or top score too low.
	if len(hits) == 0 || hits[0].Score < minScore {
		return s.refuse(ctx, queryID, userID, query, "no_sources", start, intPtr(retrievalMS))
	}

	// 5. Build prompt.
	systemPrompt, userPrompt := buildPrompt(hits, query)

	// 6. Call LLM.
	genStart := time.Now()
	result, err := s.llm.Generate(ctx, systemPrompt, userPrompt, LLMOpts{
		Temperature: 0.1,
		MaxTokens:   800,
		JSONMode:    true,
	})
	if err != nil {
		slog.Error("research LLM call failed", "query_id", queryID, "err", err)
		return nil, err
	}
	generationMS := int(time.Since(genStart).Milliseconds())

	content := strings.TrimSpace(result.Content)

	// Sentinels: LLM may return these as raw strings even in JSON mode.
	switch content {
	case "OUT_OF_SCOPE":
		return s.refuseWithTokens(ctx, queryID, userID, query, "out_of_scope", start,
			intPtr(retrievalMS), intPtr(generationMS), result.PromptTokens, result.CompletionTokens)
	case "INSUFFICIENT_SOURCES":
		return s.refuseWithTokens(ctx, queryID, userID, query, "no_sources", start,
			intPtr(retrievalMS), intPtr(generationMS), result.PromptTokens, result.CompletionTokens)
	}

	// Parse structured response.
	var llmOut struct {
		Answer      string   `json:"answer"`
		UsedChunkIDs []string `json:"used_chunk_ids"`
	}
	if err := json.Unmarshal([]byte(content), &llmOut); err != nil {
		slog.Warn("research LLM returned invalid JSON",
			"query_id", queryID, "content_prefix", truncate(content, 120))
		return s.refuseWithTokens(ctx, queryID, userID, query, "model_error", start,
			intPtr(retrievalMS), intPtr(generationMS), result.PromptTokens, result.CompletionTokens)
	}
	if llmOut.Answer == "" {
		slog.Warn("research LLM JSON missing answer field", "query_id", queryID)
		return s.refuseWithTokens(ctx, queryID, userID, query, "model_error", start,
			intPtr(retrievalMS), intPtr(generationMS), result.PromptTokens, result.CompletionTokens)
	}

	// 7. Validate citations.
	hitByID := make(map[string]data.ChunkHit, len(hits))
	for _, h := range hits {
		hitByID[h.ChunkID] = h
	}
	var badIDs []string
	for _, id := range llmOut.UsedChunkIDs {
		if _, ok := hitByID[id]; !ok {
			badIDs = append(badIDs, id)
		}
	}
	if len(badIDs) > 0 {
		slog.Warn("research LLM cited unknown chunk IDs",
			"query_id", queryID, "bad_ids", badIDs)
		return s.refuseWithTokens(ctx, queryID, userID, query, "hallucinated_citation", start,
			intPtr(retrievalMS), intPtr(generationMS), result.PromptTokens, result.CompletionTokens)
	}

	// 7b. Validate that every [N] marker in the answer text maps to a chunk
	// that is also in used_chunk_ids. Without this check the model can write
	// "revenue was X [5]" while used_chunk_ids contains an unrelated chunk —
	// the citation appears valid (chunk exists in retrieval) but does not
	// match the marker, defeating the whole point of citations.
	usedSet := make(map[string]bool, len(llmOut.UsedChunkIDs))
	for _, id := range llmOut.UsedChunkIDs {
		usedSet[id] = true
	}
	var badMarkers []string
	for _, m := range citationMarkerRe.FindAllStringSubmatch(llmOut.Answer, -1) {
		n, err := strconv.Atoi(m[1])
		if err != nil || n < 1 || n > len(hits) {
			badMarkers = append(badMarkers, m[0])
			continue
		}
		if !usedSet[hits[n-1].ChunkID] {
			badMarkers = append(badMarkers, m[0])
		}
	}
	if len(badMarkers) > 0 {
		// Cap logging to first 5 to avoid log-spam from a malicious model
		// that emits hundreds of bad markers.
		if len(badMarkers) > 5 {
			badMarkers = badMarkers[:5]
		}
		slog.Warn("research LLM emitted citation markers not in used_chunk_ids",
			"query_id", queryID, "bad_markers", badMarkers)
		return s.refuseWithTokens(ctx, queryID, userID, query, "hallucinated_citation", start,
			intPtr(retrievalMS), intPtr(generationMS), result.PromptTokens, result.CompletionTokens)
	}

	// 8. Build citations (in LLM citation order, deduped).
	seen := make(map[string]bool, len(llmOut.UsedChunkIDs))
	var citations []Citation
	for _, id := range llmOut.UsedChunkIDs {
		if seen[id] {
			continue
		}
		seen[id] = true
		h := hitByID[id]
		c := Citation{
			ChunkID:   h.ChunkID,
			SourceURL: h.SourceURL,
			Symbol:    h.Symbol,
			Excerpt:   truncate(h.Text, maxExcerptLen),
			Score:     h.Score,
		}
		if h.FiledAt != nil {
			c.FiledAt = h.FiledAt.Format("2006-01-02")
		}
		citations = append(citations, c)
	}

	// 9. Persist.
	totalMS := int(time.Since(start).Milliseconds())
	costMicros := CalcCostMicros(s.llm, result.PromptTokens, result.CompletionTokens)
	model := s.llm.Model()
	citationsJSON, _ := json.Marshal(citations)

	answerText := llmOut.Answer
	_ = s.queries.Insert(ctx, data.ResearchQuery{
		ID:               queryID,
		UserID:           nilIfEmpty(userID),
		QueryText:        query,
		AnswerText:       &answerText,
		Refused:          false,
		Citations:        citationsJSON,
		RetrievalMS:      intPtr(retrievalMS),
		GenerationMS:     intPtr(generationMS),
		TotalMS:          intPtr(totalMS),
		PromptTokens:     intPtr(result.PromptTokens),
		CompletionTokens: intPtr(result.CompletionTokens),
		CostUSDMicros:    intPtr(costMicros),
		Model:            &model,
	})

	answer := &Answer{
		QueryID:   queryID,
		Answer:    llmOut.Answer,
		Citations: citations,
		Refused:   false,
		LatencyMS: totalMS,
	}

	// 10. Cache.
	if s.answerCache != nil {
		if b, err := json.Marshal(answer); err == nil {
			_ = s.answerCache.Set(ctx, cacheKey, b, answerCacheTTL).Err()
		}
	}

	return answer, nil
}

// refuse builds a refusal Answer with no token data and persists it.
func (s *AnswerService) refuse(
	ctx context.Context,
	queryID, userID, query, reason string,
	start time.Time,
	retrievalMS *int,
) (*Answer, error) {
	return s.refuseWithTokens(ctx, queryID, userID, query, reason, start, retrievalMS, nil, 0, 0)
}

func (s *AnswerService) refuseWithTokens(
	ctx context.Context,
	queryID, userID, query, reason string,
	start time.Time,
	retrievalMS, generationMS *int,
	promptTokens, completionTokens int,
) (*Answer, error) {
	totalMS := int(time.Since(start).Milliseconds())

	row := data.ResearchQuery{
		ID:           queryID,
		UserID:       nilIfEmpty(userID),
		QueryText:    query,
		Refused:      true,
		RefusalReason: stringPtr(reason),
		Citations:    []byte("[]"),
		RetrievalMS:  retrievalMS,
		GenerationMS: generationMS,
		TotalMS:      intPtr(totalMS),
	}
	if promptTokens > 0 {
		row.PromptTokens = intPtr(promptTokens)
	}
	if completionTokens > 0 {
		row.CompletionTokens = intPtr(completionTokens)
	}
	if promptTokens > 0 || completionTokens > 0 {
		cost := CalcCostMicros(s.llm, promptTokens, completionTokens)
		row.CostUSDMicros = intPtr(cost)
		model := s.llm.Model()
		row.Model = &model
	}

	_ = s.queries.Insert(ctx, row)

	// A no_sources refusal is a dead end for the user. Attach the set of
	// tickers the corpus can actually answer for so the UI can offer
	// actionable suggestions instead. Best-effort and presentation-only —
	// not persisted to research_queries.
	var coverage *Coverage
	if reason == "no_sources" && s.coverage != nil {
		if syms := s.coverage.get(ctx); len(syms) > 0 {
			coverage = &Coverage{Symbols: syms, Examples: coverageExamples(syms)}
		}
	}

	return &Answer{
		QueryID:       queryID,
		Refused:       true,
		RefusalReason: reason,
		Coverage:      coverage,
		Citations:     []Citation{},
		LatencyMS:     totalMS,
	}, nil
}

// coverageExamples builds a couple of concrete, answerable example questions
// from the first covered symbol so the refusal card has something clickable.
func coverageExamples(symbols []string) []string {
	if len(symbols) == 0 {
		return nil
	}
	s := symbols[0]
	return []string{
		fmt.Sprintf("What risk factors did %s disclose in its latest 10-K?", s),
		fmt.Sprintf("Summarize %s's revenue segments from its most recent filing.", s),
	}
}

// generalAnswer produces an explicitly uncited answer from the model's general
// knowledge. Only reached when the user opted in (AllowGeneral) and the server
// flag is on, and only after the forward-looking gate has already run. It still
// honours OUT_OF_SCOPE so it cannot become investment advice. Not cached; the
// row is persisted like a normal answer (no distinguishing column, by design).
func (s *AnswerService) generalAnswer(ctx context.Context, queryID, userID, query string, start time.Time) (*Answer, error) {
	system := `You are a financial research assistant answering from general knowledge. You have NO source documents for this question.
- Answer concisely and factually from general knowledge.
- You have no sources, so do NOT include any citation markers like [1].
- Do not give buy/sell/hold recommendations or price predictions. If the user asks for one, reply EXACTLY: OUT_OF_SCOPE
- If you are unsure, say so plainly rather than guessing.

Output format (JSON only, no other text):
{"answer": "<your answer>"}`
	user := "QUESTION: " + query

	genStart := time.Now()
	result, err := s.llm.Generate(ctx, system, user, LLMOpts{
		Temperature: 0.2,
		MaxTokens:   800,
		JSONMode:    true,
	})
	if err != nil {
		slog.Error("research general LLM call failed", "query_id", queryID, "err", err)
		return nil, err
	}
	generationMS := int(time.Since(genStart).Milliseconds())
	content := strings.TrimSpace(result.Content)

	if content == "OUT_OF_SCOPE" {
		return s.refuseWithTokens(ctx, queryID, userID, query, "out_of_scope", start,
			nil, intPtr(generationMS), result.PromptTokens, result.CompletionTokens)
	}

	var llmOut struct {
		Answer string `json:"answer"`
	}
	if jsonErr := json.Unmarshal([]byte(content), &llmOut); jsonErr != nil || strings.TrimSpace(llmOut.Answer) == "" {
		slog.Warn("research general LLM returned invalid/empty JSON",
			"query_id", queryID, "content_prefix", truncate(content, 120))
		return s.refuseWithTokens(ctx, queryID, userID, query, "model_error", start,
			nil, intPtr(generationMS), result.PromptTokens, result.CompletionTokens)
	}

	totalMS := int(time.Since(start).Milliseconds())
	costMicros := CalcCostMicros(s.llm, result.PromptTokens, result.CompletionTokens)
	model := s.llm.Model()
	answerText := llmOut.Answer

	_ = s.queries.Insert(ctx, data.ResearchQuery{
		ID:               queryID,
		UserID:           nilIfEmpty(userID),
		QueryText:        query,
		AnswerText:       &answerText,
		Refused:          false,
		Citations:        []byte("[]"),
		GenerationMS:     intPtr(generationMS),
		TotalMS:          intPtr(totalMS),
		PromptTokens:     intPtr(result.PromptTokens),
		CompletionTokens: intPtr(result.CompletionTokens),
		CostUSDMicros:    intPtr(costMicros),
		Model:            &model,
	})

	return &Answer{
		QueryID:   queryID,
		Answer:    llmOut.Answer,
		Citations: []Citation{},
		Refused:   false,
		Mode:      "general",
		LatencyMS: totalMS,
	}, nil
}

func isForwardLooking(query string) bool {
	return forwardLookingRe.MatchString(query) || forecastTermRe.MatchString(query)
}

// buildPrompt constructs the system and user prompt strings from the retrieved hits.
func buildPrompt(hits []data.ChunkHit, query string) (system, user string) {
	system = `You are a financial research assistant. Answer ONLY using the numbered sources below.
- If the sources do not contain enough information to answer, reply EXACTLY: INSUFFICIENT_SOURCES
- Do not give buy/sell/hold recommendations or price predictions. If the user asks for one, reply EXACTLY: OUT_OF_SCOPE
- Never speculate. Never use information outside the sources.

Citation discipline (read carefully — this is graded):
- Cite each factual claim with [N] markers pointing to the numbered sources.
- Each [N] must point to a chunk that, on its own, directly substantiates the specific claim being made.
- Do NOT cite a chunk just because it is topically related. Topical relevance is not support.
- Prefer ONE citation per claim. Only add a second [N] when a second source independently supports the same specific claim.
- When in doubt, cite less, not more. An uncited sentence is better than a wrong citation.
- Do not bundle citations like [1][2][3][4][5] for one sentence — that is a hallucinated-citation failure.

Output format (JSON only, no other text):
{"answer": "<your answer with [1], [2], ... markers>", "used_chunk_ids": ["chunk_id_1", "chunk_id_2"]}`

	var sb strings.Builder
	sb.WriteString("SOURCES:\n")
	for i, h := range hits {
		sb.WriteString(fmt.Sprintf("[%d] (chunk_id: %s", i+1, h.ChunkID))
		if h.Symbol != "" {
			sb.WriteString(fmt.Sprintf(", %s", h.Symbol))
		}
		if h.FiledAt != nil {
			sb.WriteString(fmt.Sprintf(" filed %s", h.FiledAt.Format("2006-01-02")))
		}
		if h.Section != "" {
			sb.WriteString(fmt.Sprintf(`, section "%s"`, h.Section))
		}
		sb.WriteString(")\n")
		sb.WriteString(h.Text)
		sb.WriteString("\n\n")
	}
	sb.WriteString("QUESTION: ")
	sb.WriteString(query)
	user = sb.String()
	return
}

func answerCacheKey(query string, symbols []string) string {
	normalized := strings.ToLower(strings.TrimSpace(query))
	joined := strings.Join(symbols, ",")
	sum := sha256.Sum256([]byte(normalized + joined))
	return fmt.Sprintf("research:answer:%x", sum)
}

func makeQueryID(query, userID string, t time.Time) string {
	raw := fmt.Sprintf("%s|%s|%d", query, userID, t.UnixNano())
	sum := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%x", sum[:16])
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func stringPtr(s string) *string { return &s }
func intPtr(n int) *int          { return &n }

// nilIfEmpty returns nil when s is the empty string so that optional FK columns
// (e.g. research_queries.user_id) are written as SQL NULL rather than a FK
// violation when no real user is associated with the query.
func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
