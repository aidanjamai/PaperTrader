package market

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"papertrader/internal/service"
	"papertrader/internal/util"
)

// MarketServicer is the subset of service.MarketService used by StockHandler.
// Defining it here mirrors the InvestmentServicer / WatchlistServicer pattern
// in sibling packages and lets the handler be tested without a live MarketStack
// client.
type MarketServicer interface {
	GetStock(ctx context.Context, symbol string) (*service.StockData, error)
	GetHistoricalData(ctx context.Context, symbol string) (*service.HistoricalData, error)
	GetBatchHistoricalData(ctx context.Context, symbols []string) (map[string]*service.HistoricalData, error)
	GetHistoricalSeries(ctx context.Context, symbol string, days int) (*service.HistoricalSeries, error)
}

type StockHandler struct {
	service MarketServicer
}

func NewStockHandler(s MarketServicer) *StockHandler {
	return &StockHandler{service: s}
}

// Helpers
func (h *StockHandler) writeJSONResponse(w http.ResponseWriter, statusCode int, response interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}

func (h *StockHandler) writeSuccessResponse(w http.ResponseWriter, statusCode int, message string, data interface{}) {
	response := MarketResponse{
		Success: true,
		Message: message,
		Data:    data,
	}
	h.writeJSONResponse(w, statusCode, response)
}

func (h *StockHandler) writeErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	response := ErrorResponse{
		Success: false,
		Message: message,
	}
	h.writeJSONResponse(w, statusCode, response)
}

// Handler Methods

func (h *StockHandler) GetStock(w http.ResponseWriter, r *http.Request) {
	symbol := r.URL.Query().Get("symbol")

	data, err := h.service.GetStock(r.Context(), symbol)
	if err != nil {
		userMessage, statusCode, _ := util.MapServiceError(err)
		h.writeErrorResponse(w, statusCode, userMessage)
		return
	}

	h.writeSuccessResponse(w, http.StatusOK, "Stock data retrieved successfully", data)
}

func (h *StockHandler) GetStockHistoricalDataDaily(w http.ResponseWriter, r *http.Request) {
	symbol := r.URL.Query().Get("symbol")

	data, err := h.service.GetHistoricalData(r.Context(), symbol)
	if err != nil {
		slog.Warn("GetStockHistoricalDataDaily failed", "symbol", symbol, "err", err)
		userMessage, statusCode, _ := util.MapServiceError(err)
		h.writeErrorResponse(w, statusCode, userMessage)
		return
	}

	h.writeSuccessResponse(w, http.StatusOK, "Historical stock data retrieved successfully", data)
}

// GetStockHistoricalSeries returns a daily-close time series for one symbol.
// Reads ?symbol= and an optional ?days= (default 90, clamped to MaxHistoricalSeriesDays).
func (h *StockHandler) GetStockHistoricalSeries(w http.ResponseWriter, r *http.Request) {
	symbol := r.URL.Query().Get("symbol")
	days := 0
	if raw := r.URL.Query().Get("days"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			h.writeErrorResponse(w, http.StatusBadRequest, "days must be a positive integer")
			return
		}
		days = parsed
	}

	data, err := h.service.GetHistoricalSeries(r.Context(), symbol, days)
	if err != nil {
		slog.Warn("GetStockHistoricalSeries failed", "symbol", symbol, "days", days, "err", err)
		userMessage, statusCode, _ := util.MapServiceError(err)
		h.writeErrorResponse(w, statusCode, userMessage)
		return
	}

	h.writeSuccessResponse(w, http.StatusOK, "Historical series retrieved", data)
}

// GetBatchHistoricalDataDaily handles batch requests for multiple stock symbols
func (h *StockHandler) GetBatchHistoricalDataDaily(w http.ResponseWriter, r *http.Request) {
	// Get symbols from query parameter (comma-separated)
	symbolsParam := r.URL.Query().Get("symbols")
	if symbolsParam == "" {
		h.writeErrorResponse(w, http.StatusBadRequest, "symbols parameter is required (comma-separated)")
		return
	}

	// Parse comma-separated symbols
	symbols := strings.FieldsFunc(symbolsParam, func(c rune) bool {
		return c == ',' || c == ' '
	})

	if len(symbols) == 0 {
		h.writeErrorResponse(w, http.StatusBadRequest, "at least one symbol is required")
		return
	}

	// Limit batch size to prevent abuse (adjust based on your MarketStack plan)
	const maxBatchSize = 15
	if len(symbols) > maxBatchSize {
		h.writeErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("maximum %d symbols allowed per request", maxBatchSize))
		return
	}

	data, err := h.service.GetBatchHistoricalData(r.Context(), symbols)
	if err != nil {
		slog.Warn("GetBatchHistoricalDataDaily failed", "symbols", symbols, "err", err)
		userMessage, statusCode, _ := util.MapServiceError(err)
		h.writeErrorResponse(w, statusCode, userMessage)
		return
	}

	h.writeSuccessResponse(w, http.StatusOK, fmt.Sprintf("Historical data retrieved for %d symbols", len(data)), data)
}
