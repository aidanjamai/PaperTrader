package investments

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"papertrader/internal/data"
	"papertrader/internal/util"
)

// validateIdempotencyKey checks the optional Idempotency-Key header.
// Returns ("", nil) if absent. Returns an error string (non-empty) if invalid.
// The returned key is the trimmed value; downstream callers must use it instead
// of re-reading the header.
func validateIdempotencyKey(r *http.Request) (string, string) {
	raw := r.Header.Get("Idempotency-Key")
	if raw == "" {
		return "", ""
	}
	key := strings.TrimSpace(raw)
	if key == "" {
		// Original was non-empty but consisted entirely of whitespace.
		return "", "Idempotency-Key must not be blank or whitespace-only"
	}
	if len(key) > 255 {
		return "", "Idempotency-Key must be 255 characters or fewer"
	}
	for i := 0; i < len(key); i++ {
		b := key[i]
		if b < 32 || b > 126 {
			return "", "Idempotency-Key must contain only printable ASCII characters"
		}
	}
	return key, ""
}

// History query param bounds.
const (
	maxHistoryLimit     = 200
	defaultHistoryLimit = 50
)

// InvestmentServicer is the subset of service.InvestmentService used by InvestmentsHandler.
type InvestmentServicer interface {
	BuyStock(ctx context.Context, userID, symbol string, quantity int, idempotencyKey string) (*data.UserStock, error)
	SellStock(ctx context.Context, userID, symbol string, quantity int, idempotencyKey string) (*data.UserStock, error)
	GetUserStocks(ctx context.Context, userID string) ([]data.UserStock, error)
	GetUserTrades(ctx context.Context, userID string, opts data.TradeQueryOpts) ([]data.Trade, int, error)
}

type InvestmentsHandler struct {
	service InvestmentServicer
}

func NewInvestmentsHandler(s InvestmentServicer) *InvestmentsHandler {
	return &InvestmentsHandler{service: s}
}

func (h *InvestmentsHandler) BuyStock(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req BuyStockRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		util.WriteSafeError(w, http.StatusBadRequest, "Invalid request body", err, "INVALID_REQUEST")
		return
	}

	// Validate quantity
	if err := util.ValidateQuantity(req.Quantity); err != nil {
		util.WriteSafeError(w, http.StatusBadRequest, err.Error(), err, "VALIDATION_ERROR")
		return
	}

	// Validate and sanitize symbol (defense in depth)
	symbol, err := util.ValidateSymbol(req.Symbol)
	if err != nil {
		util.WriteSafeError(w, http.StatusBadRequest, err.Error(), err, "VALIDATION_ERROR")
		return
	}

	// Validate optional Idempotency-Key header
	idempotencyKey, errMsg := validateIdempotencyKey(r)
	if errMsg != "" {
		util.WriteSafeError(w, http.StatusBadRequest, errMsg, nil, "VALIDATION_ERROR")
		return
	}

	userStock, err := h.service.BuyStock(r.Context(), userID, symbol, req.Quantity, idempotencyKey)
	if err != nil {
		util.WriteServiceError(w, err)
		return
	}

	// Set Content-Type header before writing response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(userStock)
}

func (h *InvestmentsHandler) SellStock(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req SellStockRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		util.WriteSafeError(w, http.StatusBadRequest, "Invalid request body", err, "INVALID_REQUEST")
		return
	}

	// Validate quantity
	if err := util.ValidateQuantity(req.Quantity); err != nil {
		util.WriteSafeError(w, http.StatusBadRequest, err.Error(), err, "VALIDATION_ERROR")
		return
	}

	// Validate and sanitize symbol (defense in depth)
	symbol, err := util.ValidateSymbol(req.Symbol)
	if err != nil {
		util.WriteSafeError(w, http.StatusBadRequest, err.Error(), err, "VALIDATION_ERROR")
		return
	}

	// Validate optional Idempotency-Key header
	idempotencyKey, errMsg := validateIdempotencyKey(r)
	if errMsg != "" {
		util.WriteSafeError(w, http.StatusBadRequest, errMsg, nil, "VALIDATION_ERROR")
		return
	}

	userStock, err := h.service.SellStock(r.Context(), userID, symbol, req.Quantity, idempotencyKey)
	if err != nil {
		util.WriteServiceError(w, err)
		return
	}

	// Set Content-Type header before writing response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(userStock)
}

// GetTradeHistory returns a paginated, filterable list of the user's trades.
// Query params: limit (default 50, max 200), offset (>= 0), symbol (optional),
// action (optional, BUY or SELL). All params are validated; bad input → 400.
func (h *InvestmentsHandler) GetTradeHistory(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	q := r.URL.Query()

	// limit: default 50, clamped to [1, 200]
	limit := defaultHistoryLimit
	if raw := q.Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > maxHistoryLimit {
			util.WriteSafeError(w, http.StatusBadRequest, "limit must be an integer between 1 and 200", nil, "VALIDATION_ERROR")
			return
		}
		limit = parsed
	}

	// offset: default 0, must be >= 0
	offset := 0
	if raw := q.Get("offset"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 {
			util.WriteSafeError(w, http.StatusBadRequest, "offset must be a non-negative integer", nil, "VALIDATION_ERROR")
			return
		}
		offset = parsed
	}

	// symbol: optional. If provided, must pass the same validation as buy/sell.
	var symbol string
	if raw := q.Get("symbol"); raw != "" {
		s, err := util.ValidateSymbol(raw)
		if err != nil {
			util.WriteSafeError(w, http.StatusBadRequest, err.Error(), err, "VALIDATION_ERROR")
			return
		}
		symbol = s
	}

	// action: optional, must be BUY or SELL if provided
	action := q.Get("action")
	if action != "" && action != "BUY" && action != "SELL" {
		util.WriteSafeError(w, http.StatusBadRequest, "action must be BUY or SELL", nil, "VALIDATION_ERROR")
		return
	}

	trades, total, err := h.service.GetUserTrades(r.Context(), userID, data.TradeQueryOpts{
		Symbol: symbol,
		Action: action,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		util.WriteServiceError(w, err)
		return
	}

	resp := TradeHistoryResponse{
		Trades: trades,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

func (h *InvestmentsHandler) GetUserStocks(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	stocks, err := h.service.GetUserStocks(r.Context(), userID)
	if err != nil {
		util.WriteServiceError(w, err)
		return
	}

	// Set Content-Type header before writing response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(stocks)
}
