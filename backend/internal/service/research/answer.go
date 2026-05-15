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

// AnswerService orchestrates retrieval, LLM generation, and citation validation.
type AnswerService struct {
	retrieval   retriever
	llm         LLMClient
	queries     queriesStore
	answerCache *redis.Client // optional, nil-safe
}

func NewAnswerService(
	ret retriever,
	llm LLMClient,
	queries queriesStore,
	cache *redis.Client,
) *AnswerService {
	return &AnswerService{
		retrieval:   ret,
		llm:         llm,
		queries:     queries,
		answerCache: cache,
	}
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

// Answer is the top-level response type returned from Ask.
type Answer struct {
	QueryID       string     `json:"query_id"`
	Answer        string     `json:"answer"`
	Citations     []Citation `json:"citations"`
	Refused       bool       `json:"refused"`
	RefusalReason string     `json:"refusal_reason,omitempty"`
	LatencyMS     int        `json:"latency_ms"`
}

// AskOpts parameterises a single Ask call.
type AskOpts struct {
	Symbols  []string
	K        int
	MinScore float64
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

	// 1. Answer cache lookup.
	cacheKey := answerCacheKey(query, opts.Symbols)
	if s.answerCache != nil {
		if cached, err := s.answerCache.Get(ctx, cacheKey).Bytes(); err == nil {
			var a Answer
			if jsonErr := json.Unmarshal(cached, &a); jsonErr == nil {
				a.QueryID = queryID
				return &a, nil
			}
		}
	}

	// 2. Forward-looking refusal (no retrieval, no LLM).
	if isForwardLooking(query) {
		return s.refuse(ctx, queryID, userID, query, "forward_looking", start, nil)
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

	return &Answer{
		QueryID:       queryID,
		Refused:       true,
		RefusalReason: reason,
		Citations:     []Citation{},
		LatencyMS:     totalMS,
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
