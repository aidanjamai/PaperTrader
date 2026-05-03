package service

import (
	"context"
	"database/sql"

	"github.com/shopspring/decimal"

	"papertrader/internal/data"
)

// Kind constants for Discrepancy.
const (
	KindQuantityMismatch  = "quantity_mismatch"
	KindAvgPriceMismatch  = "avg_price_mismatch"
	KindOrphanPortfolio   = "orphan_portfolio"
	KindMissingPortfolio  = "missing_portfolio"
	KindNegativeLedgerQty = "negative_ledger_qty"
)

// avgEpsilon is the tolerance used when comparing average prices.
// New rows use exact decimal arithmetic and should match exactly, but rows
// written before the float64 → decimal migration may have up to 0.01 drift,
// so we keep a small allowance for those legacy rows.
var avgEpsilon = decimal.RequireFromString("0.001")

// Discrepancy describes one detected inconsistency between the trade ledger
// and the materialized portfolio table for a given (user, symbol) pair.
type Discrepancy struct {
	UserID       string          `json:"user_id"`
	Symbol       string          `json:"symbol"`
	PortfolioQty int             `json:"portfolio_qty"`
	LedgerQty    int             `json:"ledger_qty"`
	PortfolioAvg decimal.Decimal `json:"portfolio_avg"`
	LedgerAvg    decimal.Decimal `json:"ledger_avg"`
	Kind         string          `json:"kind"`
}

// ReconcileService replays the trade ledger and compares it against the
// materialized portfolio view to detect consistency drift.
type ReconcileService struct {
	db             *sql.DB
	portfolioStore *data.PortfolioStore
	tradesStore    *data.TradesStore
}

// NewReconcileService constructs a ReconcileService.
func NewReconcileService(db *sql.DB, portfolioStore *data.PortfolioStore, tradesStore *data.TradesStore) *ReconcileService {
	return &ReconcileService{
		db:             db,
		portfolioStore: portfolioStore,
		tradesStore:    tradesStore,
	}
}

// ledgerEntry holds the replayed position for a single symbol.
type ledgerEntry struct {
	qty      int
	avgPrice decimal.Decimal
}

// Reconcile replays the trade ledger for userID and compares it against the
// materialized portfolio. Returns all discrepancies; an empty slice means
// the portfolio is consistent with the ledger.
func (r *ReconcileService) Reconcile(ctx context.Context, userID string) ([]Discrepancy, error) {
	trades, err := r.tradesStore.GetAllTradesByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Replay trades in chronological order to build expected state.
	expected := map[string]*ledgerEntry{}
	for _, t := range trades {
		entry, ok := expected[t.Symbol]
		if !ok {
			entry = &ledgerEntry{}
			expected[t.Symbol] = entry
		}
		switch t.Action {
		case "BUY":
			newQty := entry.qty + t.Quantity
			if newQty > 0 {
				existingTotal := entry.avgPrice.Mul(decimal.NewFromInt(int64(entry.qty)))
				addedTotal := t.Price.Mul(decimal.NewFromInt(int64(t.Quantity)))
				entry.avgPrice = existingTotal.Add(addedTotal).Div(decimal.NewFromInt(int64(newQty)))
			}
			entry.qty = newQty
		case "SELL":
			entry.qty -= t.Quantity
			// avg price unchanged on sell
			if entry.qty == 0 {
				delete(expected, t.Symbol)
			}
		}
	}

	// Fetch actual portfolio.
	holdings, err := r.portfolioStore.GetPortfolioByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	actual := map[string]data.UserStock{}
	for _, h := range holdings {
		actual[h.Symbol] = h
	}

	var discrepancies []Discrepancy

	// Check expected symbols against actual.
	for sym, exp := range expected {
		if exp.qty < 0 {
			// More SELLs than BUYs in the ledger — a real anomaly.
			discrepancies = append(discrepancies, Discrepancy{
				UserID:    userID,
				Symbol:    sym,
				LedgerQty: exp.qty,
				Kind:      KindNegativeLedgerQty,
			})
			continue
		}
		if exp.qty == 0 {
			continue
		}
		act, found := actual[sym]
		if !found {
			discrepancies = append(discrepancies, Discrepancy{
				UserID:       userID,
				Symbol:       sym,
				PortfolioQty: 0,
				LedgerQty:    exp.qty,
				PortfolioAvg: decimal.Zero,
				LedgerAvg:    exp.avgPrice,
				Kind:         KindMissingPortfolio,
			})
			continue
		}
		if act.Quantity != exp.qty {
			discrepancies = append(discrepancies, Discrepancy{
				UserID:       userID,
				Symbol:       sym,
				PortfolioQty: act.Quantity,
				LedgerQty:    exp.qty,
				PortfolioAvg: act.AvgPrice,
				LedgerAvg:    exp.avgPrice,
				Kind:         KindQuantityMismatch,
			})
		} else if act.AvgPrice.Sub(exp.avgPrice).Abs().GreaterThan(avgEpsilon) {
			discrepancies = append(discrepancies, Discrepancy{
				UserID:       userID,
				Symbol:       sym,
				PortfolioQty: act.Quantity,
				LedgerQty:    exp.qty,
				PortfolioAvg: act.AvgPrice,
				LedgerAvg:    exp.avgPrice,
				Kind:         KindAvgPriceMismatch,
			})
		}
	}

	// Check actual symbols not in expected — orphan portfolio entries.
	for sym, act := range actual {
		if _, ok := expected[sym]; !ok {
			discrepancies = append(discrepancies, Discrepancy{
				UserID:       userID,
				Symbol:       sym,
				PortfolioQty: act.Quantity,
				LedgerQty:    0,
				PortfolioAvg: act.AvgPrice,
				LedgerAvg:    decimal.Zero,
				Kind:         KindOrphanPortfolio,
			})
		}
	}

	return discrepancies, nil
}

// ReconcileAll runs Reconcile across every user that has at least one trade
// or one portfolio row. Skips users with no discrepancies from the result map.
func (r *ReconcileService) ReconcileAll(ctx context.Context) (map[string][]Discrepancy, error) {
	query := `
		SELECT DISTINCT user_id FROM trades
		UNION
		SELECT DISTINCT user_id FROM portfolio`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var userIDs []string
	for rows.Next() {
		var uid string
		if err := rows.Scan(&uid); err != nil {
			return nil, err
		}
		userIDs = append(userIDs, uid)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	result := map[string][]Discrepancy{}
	for _, uid := range userIDs {
		discrepancies, err := r.Reconcile(ctx, uid)
		if err != nil {
			return nil, err
		}
		if len(discrepancies) > 0 {
			result[uid] = discrepancies
		}
	}
	return result, nil
}
