package research

import (
	"context"
	"encoding/json"
	"net/http"

	"papertrader/internal/api/auth"
	svcresearch "papertrader/internal/service/research"
	"papertrader/internal/util"
)

const (
	maxQueryLen    = 2000
	maxSymbolCount = 50
)

// answerer is the subset of svcresearch.AnswerService the handler depends on.
// Extracting this interface keeps the handler testable without a full service.
type answerer interface {
	Ask(ctx context.Context, userID, query string, opts svcresearch.AskOpts) (*svcresearch.Answer, error)
}

// Handler handles research HTTP requests.
type Handler struct {
	svc answerer
}

func NewHandler(svc answerer) *Handler {
	return &Handler{svc: svc}
}

// askRequest is the decoded body for POST /api/research/ask.
type askRequest struct {
	Query    string   `json:"query"`
	Symbols  []string `json:"symbols"`
	K        int      `json:"k"`
	MinScore float64  `json:"min_score"`
}

// Ask handles POST /api/research/ask.
func (h *Handler) Ask(w http.ResponseWriter, r *http.Request) {
	// Read user identity from the request context populated by auth.JWTMiddleware,
	// not from the X-User-ID header. The header pattern is safe only when
	// StripUserHeaders is wired globally; reading from context fails closed if
	// JWT middleware is missing from this route, even if the header scrubber is
	// not in the chain.
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req askRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		util.WriteSafeError(w, http.StatusBadRequest, "invalid request body", err, "INVALID_REQUEST")
		return
	}

	if req.Query == "" {
		util.WriteSafeError(w, http.StatusBadRequest, "query is required", nil, "VALIDATION_ERROR")
		return
	}
	if len(req.Query) > maxQueryLen {
		util.WriteSafeError(w, http.StatusBadRequest, "query exceeds maximum length of 2000 characters", nil, "VALIDATION_ERROR")
		return
	}
	if len(req.Symbols) > maxSymbolCount {
		util.WriteSafeError(w, http.StatusBadRequest, "symbols list exceeds maximum of 50 entries", nil, "VALIDATION_ERROR")
		return
	}

	answer, err := h.svc.Ask(r.Context(), userID, req.Query, svcresearch.AskOpts{
		Symbols:  req.Symbols,
		K:        req.K,
		MinScore: req.MinScore,
	})
	if err != nil {
		util.WriteSafeError(w, http.StatusInternalServerError, "research query failed", err, "INTERNAL_ERROR")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(answer)
}
