//go:build integration

package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"papertrader/internal/data"
	"papertrader/internal/testutil"
)

// TestReconcile_DetectsDriftAfterDirectPortfolioMutation proves the
// reconciliation service correctly identifies a drift between the trade ledger
// and the materialized portfolio table when the portfolio is mutated directly
// (bypassing the service layer).
//
// Flow:
//  1. Create a real user and execute 10 BuyStock + 2 SellStock through the
//     real InvestmentService so that trades and portfolio remain consistent.
//  2. Run Reconcile — expect zero discrepancies.
//  3. Directly UPDATE portfolio SET quantity = quantity + 1 for AAPL (the
//     append-only trigger is on `trades`, not `portfolio`, so this succeeds).
//  4. Run Reconcile again — expect exactly one Discrepancy with
//     Kind == "quantity_mismatch" for AAPL.
func TestReconcile_DetectsDriftAfterDirectPortfolioMutation(t *testing.T) {
	db := testutil.NewIntegrationDB(t)
	testutil.Truncate(t, db, "trades", "portfolio", "users")

	// --- Setup: create user with enough balance for 10 buys at $100 each = $1000 ---
	userID := uuid.New().String()
	_, err := db.Exec(
		`INSERT INTO users (id, email, password, balance, email_verified, created_via)
		 VALUES ($1, $2, 'testhash', 10000.00, TRUE, 'email')`,
		userID, fmt.Sprintf("reconcile-%s@example.com", userID[:8]),
	)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	market := &integrationMarket{symbol: "AAPL", price: decimal.NewFromFloat(100.0)}
	portfolioStore := data.NewPortfolioStore(db)
	tradesStore := data.NewTradesStore(db)
	svc := NewInvestmentService(db, market, portfolioStore, tradesStore)
	reconcileSvc := NewReconcileService(db, portfolioStore, tradesStore)

	// Execute 10 BuyStock calls for AAPL (each with a unique idempotency key).
	for i := 0; i < 10; i++ {
		ikey := fmt.Sprintf("buy-setup-%d-%s", i, userID[:8])
		if _, err := svc.BuyStock(context.Background(), userID, "AAPL", 1, ikey); err != nil {
			t.Fatalf("BuyStock %d: %v", i, err)
		}
	}

	// Execute 2 SellStock calls for AAPL.
	for i := 0; i < 2; i++ {
		ikey := fmt.Sprintf("sell-setup-%d-%s", i, userID[:8])
		if _, err := svc.SellStock(context.Background(), userID, "AAPL", 1, ikey); err != nil {
			t.Fatalf("SellStock %d: %v", i, err)
		}
	}
	// Net AAPL position: 10 buys - 2 sells = 8 shares at $100 avg.

	// --- Step 1: Reconcile must be clean after legitimate service operations ---
	discrepancies, err := reconcileSvc.Reconcile(context.Background(), userID)
	if err != nil {
		t.Fatalf("Reconcile (before drift): %v", err)
	}
	if len(discrepancies) != 0 {
		t.Fatalf("expected no discrepancies before drift, got %d: %+v",
			len(discrepancies), discrepancies)
	}

	// --- Step 2: Directly mutate the portfolio (bypassing the service layer) ---
	// The append-only trigger is only on `trades`; portfolio rows are freely
	// updatable. This simulates the kind of drift the reconciler is designed to
	// catch (e.g. a manual hotfix, a bug in a non-transactional code path, or
	// a failed partial migration).
	result, err := db.Exec(
		`UPDATE portfolio SET quantity = quantity + 1 WHERE user_id = $1 AND symbol = 'AAPL'`,
		userID,
	)
	if err != nil {
		t.Fatalf("direct portfolio mutation: %v", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		t.Fatalf("RowsAffected: %v", err)
	}
	if rowsAffected != 1 {
		t.Fatalf("expected 1 portfolio row updated, got %d", rowsAffected)
	}

	// --- Step 3: Reconcile must now detect exactly one quantity mismatch ---
	discrepancies, err = reconcileSvc.Reconcile(context.Background(), userID)
	if err != nil {
		t.Fatalf("Reconcile (after drift): %v", err)
	}
	if len(discrepancies) != 1 {
		t.Fatalf("expected exactly 1 discrepancy after drift, got %d: %+v",
			len(discrepancies), discrepancies)
	}

	d := discrepancies[0]
	if d.Kind != KindQuantityMismatch {
		t.Errorf("discrepancy kind: got %q, want %q", d.Kind, KindQuantityMismatch)
	}
	if d.Symbol != "AAPL" {
		t.Errorf("discrepancy symbol: got %q, want AAPL", d.Symbol)
	}
	// Ledger says 8; portfolio now says 9 (manually incremented).
	if d.LedgerQty != 8 {
		t.Errorf("LedgerQty: got %d, want 8", d.LedgerQty)
	}
	if d.PortfolioQty != 9 {
		t.Errorf("PortfolioQty: got %d, want 9", d.PortfolioQty)
	}
}
