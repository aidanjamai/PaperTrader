package market

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"papertrader/internal/service"
	"papertrader/internal/util"
)

type StockHandler struct {
	service *service.MarketService
}

func NewStockHandler(s *service.MarketService) *StockHandler {
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

	data, err := h.service.GetStock(symbol)
	if err != nil {
		userMessage, statusCode, _ := util.MapServiceError(err)
		h.writeErrorResponse(w, statusCode, userMessage)
		return
	}

	h.writeSuccessResponse(w, http.StatusOK, "Stock data retrieved successfully", data)
}

func (h *StockHandler) PostStock(w http.ResponseWriter, r *http.Request) {
	var req PostStockRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid JSON format")
		return
	}

	if err := h.service.SaveStock(req.Symbol, req.Price); err != nil {
		userMessage, statusCode, _ := util.MapServiceError(err)
		h.writeErrorResponse(w, statusCode, userMessage)
		return
	}

	h.writeSuccessResponse(w, http.StatusOK, "Stock data saved successfully", req)
}

func (h *StockHandler) DeleteStockBySymbol(w http.ResponseWriter, r *http.Request) {
	var req StockSymbolRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid JSON format")
		return
	}

	err := h.service.DeleteStockBySymbol(req.Symbol)
	if err != nil {
		userMessage, statusCode, _ := util.MapServiceError(err)
		h.writeErrorResponse(w, statusCode, userMessage)
		return
	}

	h.writeSuccessResponse(w, http.StatusOK, "Stock cache invalidated successfully", nil)
}

func (h *StockHandler) GetStockHistoricalDataDaily(w http.ResponseWriter, r *http.Request) {
	symbol := r.URL.Query().Get("symbol")

	data, err := h.service.GetHistoricalData(symbol)
	if err != nil {
		log.Printf("GetStockHistoricalDataDaily error: %v", err)
		userMessage, statusCode, _ := util.MapServiceError(err)
		h.writeErrorResponse(w, statusCode, userMessage)
		return
	}

	h.writeSuccessResponse(w, http.StatusOK, "Historical stock data retrieved successfully", data)
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

	data, err := h.service.GetBatchHistoricalData(symbols)
	if err != nil {
		log.Printf("GetBatchHistoricalDataDaily error: %v", err)
		userMessage, statusCode, _ := util.MapServiceError(err)
		h.writeErrorResponse(w, statusCode, userMessage)
		return
	}

	h.writeSuccessResponse(w, http.StatusOK, fmt.Sprintf("Historical data retrieved for %d symbols", len(data)), data)
}
