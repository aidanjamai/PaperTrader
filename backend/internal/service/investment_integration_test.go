//go:build integration

package service

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"papertrader/internal/data"
	"papertrader/internal/testutil"
)

// integrationMarket is a MarketPricer stub that returns a fixed price.
// It is used only in integration tests so that no live MarketStack API call
// is made from CI.
type integrationMarket struct {
	symbol string
	price  decimal.Decimal
}

func (m *integrationMarket) GetStock(_ context.Context, _ string) (*StockData, error) {
	return &StockData{Symbol: m.symbol, Price: m.price}, nil
}

func (m *integrationMarket) GetBatchHistoricalData(_ context.Context, _ []string) (map[string]*HistoricalData, error) {
	return nil, nil
}

// TestBuyStock_RollsBackOnPortfolioFailure verifies that when the portfolio
// upsert fails mid-transaction, neither the balance debit nor the trade insert
// is committed — i.e., the transaction rolls back atomically.
//
// Failure injection strategy: InvestmentService constructs a fresh
// data.NewPortfolioStore(tx) inside BuyStock (line ~78 of investment.go), so
// swapping the portfolioStore field on the service struct does NOT intercept
// the in-transaction portfolio call. To force a portfolio failure without
// modifying production code we temporarily add a CHECK constraint to the
// portfolio table that rejects all rows for our test user ID, then remove it
// after the call. This is test-only DDL — no production code is changed.
func TestBuyStock_RollsBackOnPortfolioFailure(t *testing.T) {
	db := testutil.NewIntegrationDB(t)
	testutil.Truncate(t, db, "trades", "portfolio", "users")

	// Create test user with $1000 balance via direct INSERT (avoids bcrypt cost).
	userID := uuid.New().String()
	userStore := data.NewUserStore(db)
	_, err := db.Exec(
		`INSERT INTO users (id, email, password, balance, email_verified, created_via)
		 VALUES ($1, $2, 'testhash', 1000.00, TRUE, 'email')`,
		userID, fmt.Sprintf("rollback-%s@example.com", userID[:8]),
	)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	// Add a CHECK constraint that makes portfolio inserts for this user fail.
	// The constraint name is test-scoped so it won't collide with other tests.
	constraintName := "chk_test_reject_user_" + userID[:8]
	addConstraint := fmt.Sprintf(
		`ALTER TABLE portfolio ADD CONSTRAINT %s CHECK (user_id <> '%s')`,
		constraintName, userID,
	)
	if _, err := db.Exec(addConstraint); err != nil {
		t.Fatalf("add portfolio check constraint: %v", err)
	}
	t.Cleanup(func() {
		dropConstraint := fmt.Sprintf(
			`ALTER TABLE portfolio DROP CONSTRAINT IF EXISTS %s`, constraintName,
		)
		if _, err := db.Exec(dropConstraint); err != nil {
			t.Logf("warning: failed to drop test constraint %s: %v", constraintName, err)
		}
	})

	market := &integrationMarket{symbol: "AAPL", price: decimal.NewFromFloat(100.0)}
	svc := NewInvestmentService(
		db,
		market,
		data.NewPortfolioStore(db),
		data.NewTradesStore(db),
	)

	// BuyStock must fail because the portfolio upsert trips the check constraint.
	_, err = svc.BuyStock(context.Background(), userID, "AAPL", 1, "")
	if err == nil {
		t.Fatal("expected BuyStock to return an error when portfolio upsert fails, got nil")
	}

	// (1) Balance must be unchanged — no debit should have been committed.
	balance, err := userStore.GetBalance(context.Background(), userID)
	if err != nil {
		t.Fatalf("GetBalance: %v", err)
	}
	wantBalance := decimal.NewFromFloat(1000.0)
	if !balance.Equal(wantBalance) {
		t.Errorf("balance: got %s, want 1000.00 (debit must have rolled back)", balance)
	}

	// (2) No trade row must have been inserted.
	var count int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM trades WHERE user_id = $1`, userID,
	).Scan(&count); err != nil {
		t.Fatalf("count trades: %v", err)
	}
	if count != 0 {
		t.Errorf("trade count: got %d, want 0 (trade insert must have rolled back)", count)
	}
}

// TestBuyStock_ConcurrentSameIdempotencyKey fires 5 goroutines all calling
// BuyStock with the same idempotency key. The Phase 2 unique index on
// (user_id, idempotency_key) guarantees exactly one trade row is written and
// the user's balance is debited exactly once. All 5 goroutines must return
// a non-nil *data.UserStock with the same symbol and quantity.
func TestBuyStock_ConcurrentSameIdempotencyKey(t *testing.T) {
	db := testutil.NewIntegrationDB(t)
	testutil.Truncate(t, db, "trades", "portfolio", "users")

	userID := uuid.New().String()
	userStore := data.NewUserStore(db)
	_, err := db.Exec(
		`INSERT INTO users (id, email, password, balance, email_verified, created_via)
		 VALUES ($1, $2, 'testhash', 10000.00, TRUE, 'email')`,
		userID, fmt.Sprintf("concurrent-%s@example.com", userID[:8]),
	)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	market := &integrationMarket{symbol: "AAPL", price: decimal.NewFromFloat(100.0)}
	svc := NewInvestmentService(
		db,
		market,
		data.NewPortfolioStore(db),
		data.NewTradesStore(db),
	)

	const goroutines = 5
	const idempotencyKey = "shared-key-abc"

	type result struct {
		stock *data.UserStock
		err   error
	}
	results := make([]result, goroutines)

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		i := i
		go func() {
			defer wg.Done()
			stock, err := svc.BuyStock(context.Background(), userID, "AAPL", 1, idempotencyKey)
			results[i] = result{stock: stock, err: err}
		}()
	}
	wg.Wait()

	// (1) Exactly one trade row must exist for this key.
	var tradeCount int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM trades WHERE user_id = $1 AND idempotency_key = $2`,
		userID, idempotencyKey,
	).Scan(&tradeCount); err != nil {
		t.Fatalf("count trades: %v", err)
	}
	if tradeCount != 1 {
		t.Errorf("trade count: got %d, want exactly 1", tradeCount)
	}

	// (2) Balance must have been debited exactly once: 10000 - (100 * 1) = 9900.
	balance, err := userStore.GetBalance(context.Background(), userID)
	if err != nil {
		t.Fatalf("GetBalance: %v", err)
	}
	wantBalance2 := decimal.NewFromFloat(9900.0)
	if !balance.Equal(wantBalance2) {
		t.Errorf("balance: got %s, want 9900.00 (debited exactly once)", balance)
	}

	// (3) All 5 callers must receive a non-nil *data.UserStock.
	// Per spec: "all 5 returns are identical." The re-fetch after a 23505
	// collision must always find the committed row under Postgres serialization,
	// so any error here is a genuine failure of the idempotency contract.
	for i, r := range results {
		if r.err != nil {
			t.Errorf("goroutine %d returned error: %v", i, r.err)
			continue
		}
		if r.stock == nil {
			t.Errorf("goroutine %d: got nil stock with nil error", i)
			continue
		}
		if r.stock.Symbol != "AAPL" {
			t.Errorf("goroutine %d: symbol got %q, want AAPL", i, r.stock.Symbol)
		}
	}

	// (4) All successful callers must agree on the quantity field.
	var firstStock *data.UserStock
	for _, r := range results {
		if r.err == nil && r.stock != nil {
			firstStock = r.stock
			break
		}
	}
	if firstStock != nil {
		for i, r := range results {
			if r.err != nil || r.stock == nil {
				continue
			}
			if r.stock.Quantity != firstStock.Quantity {
				t.Errorf("goroutine %d: quantity %d disagrees with first caller's %d",
					i, r.stock.Quantity, firstStock.Quantity)
			}
		}
	}
}
