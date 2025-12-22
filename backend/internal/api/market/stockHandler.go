package market

import (
	"encoding/json"
	"log"
	"net/http"

	"papertrader/internal/service"
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
		h.writeErrorResponse(w, http.StatusInternalServerError, err.Error())
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
		h.writeErrorResponse(w, http.StatusInternalServerError, err.Error())
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
		h.writeErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeSuccessResponse(w, http.StatusOK, "Stock cache invalidated successfully", nil)
}

func (h *StockHandler) GetStockHistoricalDataDaily(w http.ResponseWriter, r *http.Request) {
	symbol := r.URL.Query().Get("symbol")

	data, err := h.service.GetHistoricalData(symbol)
	if err != nil {
		log.Printf("GetStockHistoricalDataDaily error: %v", err)
		h.writeErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeSuccessResponse(w, http.StatusOK, "Historical stock data retrieved successfully", data)
}
