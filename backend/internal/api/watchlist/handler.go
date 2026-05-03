package watchlist

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gorilla/mux"

	"papertrader/internal/data"
	"papertrader/internal/service"
	"papertrader/internal/util"
)

type WatchlistServicer interface {
	AddSymbol(ctx context.Context, userID, symbol string) (*service.WatchlistEntryView, error)
	RemoveSymbol(ctx context.Context, userID, symbol string) error
	List(ctx context.Context, userID string) ([]service.WatchlistEntryView, error)
}

type WatchlistHandler struct {
	service WatchlistServicer
}

func NewWatchlistHandler(s WatchlistServicer) *WatchlistHandler {
	return &WatchlistHandler{service: s}
}

func (h *WatchlistHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	items, err := h.service.List(r.Context(), userID)
	if err != nil {
		util.WriteServiceError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(ListResponse{Items: items})
}

func (h *WatchlistHandler) Add(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req AddRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		util.WriteSafeError(w, http.StatusBadRequest, "Invalid request body", err, "INVALID_REQUEST")
		return
	}

	entry, err := h.service.AddSymbol(r.Context(), userID, req.Symbol)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrSymbolNotFound):
			util.WriteSafeError(w, http.StatusNotFound, "Symbol not found", err, "SYMBOL_NOT_FOUND")
		case errors.Is(err, data.ErrWatchlistEntryExists):
			util.WriteSafeError(w, http.StatusConflict, "Symbol already in watchlist", err, "WATCHLIST_DUPLICATE")
		default:
			util.WriteServiceError(w, err)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(entry)
}

func (h *WatchlistHandler) Remove(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	symbol := mux.Vars(r)["symbol"]
	if err := h.service.RemoveSymbol(r.Context(), userID, symbol); err != nil {
		if errors.Is(err, data.ErrWatchlistEntryNotFound) {
			util.WriteSafeError(w, http.StatusNotFound, "Watchlist entry not found", err, "WATCHLIST_NOT_FOUND")
			return
		}
		util.WriteServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
