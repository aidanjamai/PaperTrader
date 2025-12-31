package investments

import (
	"encoding/json"
	"net/http"

	"papertrader/internal/service"
	"papertrader/internal/util"
)

type InvestmentsHandler struct {
	service *service.InvestmentService
}

func NewInvestmentsHandler(s *service.InvestmentService) *InvestmentsHandler {
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

	userStock, err := h.service.BuyStock(userID, symbol, req.Quantity)
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

	userStock, err := h.service.SellStock(userID, symbol, req.Quantity)
	if err != nil {
		util.WriteServiceError(w, err)
		return
	}

	// Set Content-Type header before writing response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(userStock)
}

func (h *InvestmentsHandler) GetUserStocks(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	stocks, err := h.service.GetUserStocks(userID)
	if err != nil {
		util.WriteServiceError(w, err)
		return
	}

	// Set Content-Type header before writing response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(stocks)
}
