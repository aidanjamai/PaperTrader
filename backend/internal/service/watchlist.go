package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/shopspring/decimal"

	"papertrader/internal/data"
	"papertrader/internal/util"
)

// WatchlistMarket is the subset of MarketService used by WatchlistService.
type WatchlistMarket interface {
	GetBatchHistoricalData(ctx context.Context, symbols []string) (map[string]*HistoricalData, error)
}

// WatchlistEntryView is a watchlist entry enriched with current price information.
// Price/Change/ChangePercentage are zero when the price lookup is unavailable
// (e.g. MarketStack is down) — clients should render "—" in that case.
type WatchlistEntryView struct {
	ID               string          `json:"id"`
	Symbol           string          `json:"symbol"`
	CreatedAt        string          `json:"created_at"`
	Price            decimal.Decimal `json:"price"`
	Change           decimal.Decimal `json:"change"`
	ChangePercentage decimal.Decimal `json:"change_percentage"`
	HasPrice         bool            `json:"has_price"`
}

// ErrSymbolNotFound is declared in errors.go alongside other typed service
// errors so MapServiceError can pick up its HTTPError implementation.

type WatchlistService struct {
	store         *data.WatchlistStore
	marketService WatchlistMarket
}

func NewWatchlistService(store *data.WatchlistStore, marketService WatchlistMarket) *WatchlistService {
	return &WatchlistService{store: store, marketService: marketService}
}

// AddSymbol validates the symbol against MarketStack and inserts it.
// Returns ErrSymbolNotFound if MarketStack has no data for the symbol.
// Returns data.ErrWatchlistEntryExists if the user already watches it.
func (s *WatchlistService) AddSymbol(ctx context.Context, userID, rawSymbol string) (*WatchlistEntryView, error) {
	symbol, err := util.ValidateSymbol(rawSymbol)
	if err != nil {
		return nil, err
	}

	priced, err := s.marketService.GetBatchHistoricalData(ctx, []string{symbol})
	if err != nil {
		return nil, fmt.Errorf("failed to verify symbol: %w", err)
	}
	hist, ok := priced[symbol]
	if !ok || hist == nil {
		return nil, ErrSymbolNotFound
	}

	entry, err := s.store.Add(ctx, userID, symbol)
	if err != nil {
		return nil, err
	}

	return &WatchlistEntryView{
		ID:               entry.ID,
		Symbol:           entry.Symbol,
		CreatedAt:        entry.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		Price:            hist.Price,
		Change:           hist.Change,
		ChangePercentage: hist.ChangePercentage,
		HasPrice:         true,
	}, nil
}

// RemoveSymbol deletes the entry. Returns data.ErrWatchlistEntryNotFound if missing.
func (s *WatchlistService) RemoveSymbol(ctx context.Context, userID, rawSymbol string) error {
	symbol, err := util.ValidateSymbol(rawSymbol)
	if err != nil {
		return err
	}
	return s.store.Remove(ctx, userID, symbol)
}

// List returns the user's watchlist enriched with current prices.
// Entries without a price lookup still appear (HasPrice=false).
func (s *WatchlistService) List(ctx context.Context, userID string) ([]WatchlistEntryView, error) {
	entries, err := s.store.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return []WatchlistEntryView{}, nil
	}

	symbols := make([]string, 0, len(entries))
	for _, e := range entries {
		symbols = append(symbols, e.Symbol)
	}

	// Best-effort price enrichment — if the API is down, return entries without prices.
	priced, err := s.marketService.GetBatchHistoricalData(ctx, symbols)
	if err != nil {
		slog.Warn("watchlist price enrichment failed", "user_id", userID, "symbol_count", len(symbols), "err", err, "component", "watchlist")
		priced = nil
	}

	views := make([]WatchlistEntryView, 0, len(entries))
	for _, e := range entries {
		view := WatchlistEntryView{
			ID:        e.ID,
			Symbol:    e.Symbol,
			CreatedAt: e.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
		if hist, ok := priced[e.Symbol]; ok && hist != nil {
			view.Price = hist.Price
			view.Change = hist.Change
			view.ChangePercentage = hist.ChangePercentage
			view.HasPrice = true
		}
		views = append(views, view)
	}
	return views, nil
}
