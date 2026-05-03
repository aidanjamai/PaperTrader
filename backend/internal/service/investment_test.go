package service

import (
	"context"
	"errors"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/lib/pq"
	"github.com/shopspring/decimal"

	"papertrader/internal/data"
)

// mockMarket implements MarketPricer for tests.
type mockMarket struct {
	stock    *StockData
	stockErr error
}

func (m *mockMarket) GetStock(_ context.Context, _ string) (*StockData, error) {
	return m.stock, m.stockErr
}

func (m *mockMarket) GetBatchHistoricalData(_ context.Context, _ []string) (map[string]*HistoricalData, error) {
	return nil, nil
}

// userCols are the columns returned by GetUserByID.
var userCols = []string{
	"id", "email", "password", "created_at", "balance",
	"email_verified", "verification_token", "verification_token_expires",
	"google_id", "created_via",
}

// balanceCols are the columns returned by GetBalanceForUpdate.
var balanceCols = []string{"balance"}

// portfolioCols are the columns returned by GetPortfolioBySymbol.
var portfolioCols = []string{
	"id", "user_id", "symbol", "quantity", "avg_price", "created_at", "updated_at",
}

func newUserRow(balance decimal.Decimal) *sqlmock.Rows {
	return sqlmock.NewRows(userCols).AddRow(
		"user-1", "test@example.com", "hashed", time.Now(), balance,
		true, nil, nil, nil, "email",
	)
}

func newBalanceRow(balance decimal.Decimal) *sqlmock.Rows {
	return sqlmock.NewRows(balanceCols).AddRow(balance)
}

// ---- BuyStock tests ----

func TestBuyStock_InvalidQuantity(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	svc := NewInvestmentService(db, &mockMarket{stock: &StockData{Symbol: "AAPL", Price: decimal.NewFromInt(100)}}, data.NewPortfolioStore(db), data.NewTradesStore(db))

	_, err = svc.BuyStock(context.Background(), "user-1", "AAPL", 0, "")
	if err == nil {
		t.Error("expected error for quantity 0, got nil")
	}
}

func TestBuyStock_MarketError(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	market := &mockMarket{stockErr: errors.New("marketstack unavailable")}
	svc := NewInvestmentService(db, market, data.NewPortfolioStore(db), data.NewTradesStore(db))

	_, err = svc.BuyStock(context.Background(), "user-1", "AAPL", 1, "")
	if err == nil || err.Error() != "marketstack unavailable" {
		t.Errorf("expected market error, got %v", err)
	}
}

func TestBuyStock_InsufficientFunds(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	// AAPL at $200, user has $50
	market := &mockMarket{stock: &StockData{Symbol: "AAPL", Price: decimal.NewFromInt(200)}}
	svc := NewInvestmentService(db, market, data.NewPortfolioStore(db), data.NewTradesStore(db))

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT balance FROM users WHERE id = \\$1 FOR UPDATE").
		WithArgs("user-1").
		WillReturnRows(newBalanceRow(decimal.NewFromFloat(50.0)))
	mock.ExpectRollback()

	_, err = svc.BuyStock(context.Background(), "user-1", "AAPL", 1, "")
	if err == nil || err.Error() != "insufficient funds" {
		t.Errorf("expected 'insufficient funds', got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled sql expectations: %v", err)
	}
}

// ---- SellStock tests ----

func TestSellStock_InvalidQuantity(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	svc := NewInvestmentService(db, &mockMarket{stock: &StockData{Symbol: "AAPL", Price: decimal.NewFromInt(150)}}, data.NewPortfolioStore(db), data.NewTradesStore(db))

	_, err = svc.SellStock(context.Background(), "user-1", "AAPL", 0, "")
	if err == nil {
		t.Error("expected error for quantity 0, got nil")
	}
}

func TestSellStock_MarketError(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	market := &mockMarket{stockErr: errors.New("API timeout")}
	svc := NewInvestmentService(db, market, data.NewPortfolioStore(db), data.NewTradesStore(db))

	_, err = svc.SellStock(context.Background(), "user-1", "AAPL", 1, "")
	if err == nil || err.Error() != "API timeout" {
		t.Errorf("expected market error, got %v", err)
	}
}

func TestSellStock_HoldingNotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	market := &mockMarket{stock: &StockData{Symbol: "TSLA", Price: decimal.NewFromInt(300)}}
	svc := NewInvestmentService(db, market, data.NewPortfolioStore(db), data.NewTradesStore(db))

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id, user_id, symbol").
		WithArgs("user-1", "TSLA").
		WillReturnRows(sqlmock.NewRows(portfolioCols)) // empty result → ErrNoRows
	mock.ExpectRollback()

	_, err = svc.SellStock(context.Background(), "user-1", "TSLA", 1, "")
	if err == nil {
		t.Error("expected error for holding not found, got nil")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled sql expectations: %v", err)
	}
}

func TestSellStock_InsufficientQuantity(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	market := &mockMarket{stock: &StockData{Symbol: "AAPL", Price: decimal.NewFromInt(150)}}
	svc := NewInvestmentService(db, market, data.NewPortfolioStore(db), data.NewTradesStore(db))

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id, user_id, symbol").
		WithArgs("user-1", "AAPL").
		WillReturnRows(sqlmock.NewRows(portfolioCols).AddRow(
			"port-1", "user-1", "AAPL", 2, decimal.NewFromInt(100), time.Now(), time.Now(),
		))
	mock.ExpectRollback()

	_, err = svc.SellStock(context.Background(), "user-1", "AAPL", 5, "") // wants 5, has 2
	if err == nil || err.Error() != "insufficient stock quantity" {
		t.Errorf("expected 'insufficient stock quantity', got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled sql expectations: %v", err)
	}
}

// ---- Idempotency tests ----

// tradeCols mirrors the columns returned by GetTradeByIdempotencyKey.
var idempColsCols = []string{
	"id", "user_id", "symbol", "action", "quantity", "price", "total", "executed_at", "status", "idempotency_key",
}

func TestBuyStock_IdempotencyReplay(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	market := &mockMarket{stock: &StockData{Symbol: "AAPL", Price: decimal.NewFromInt(150)}}
	portfolioStore := data.NewPortfolioStore(db)
	tradesStore := data.NewTradesStore(db)
	svc := NewInvestmentService(db, market, portfolioStore, tradesStore)

	executedAt := time.Now()
	// First call: GetTradeByIdempotencyKey returns existing trade
	mock.ExpectQuery("SELECT id, user_id, symbol").
		WithArgs("user-1", "idempkey-1").
		WillReturnRows(sqlmock.NewRows(idempColsCols).AddRow(
			"trade-existing", "user-1", "AAPL", "BUY", 5, decimal.NewFromInt(150), decimal.NewFromInt(750), executedAt, "COMPLETED",
			"idempkey-1",
		))
	// GetPortfolioBySymbol for replay
	mock.ExpectQuery("SELECT id, user_id, symbol").
		WithArgs("user-1", "AAPL").
		WillReturnRows(sqlmock.NewRows(portfolioCols).AddRow(
			"port-1", "user-1", "AAPL", 5, decimal.NewFromInt(150), executedAt, executedAt,
		))

	result, err := svc.BuyStock(context.Background(), "user-1", "AAPL", 5, "idempkey-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.Symbol != "AAPL" {
		t.Errorf("expected AAPL stock, got %+v", result)
	}
	// No BEGIN should have been issued
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled sql expectations: %v", err)
	}
}

func TestSellStock_IdempotencyReplay(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	market := &mockMarket{stock: &StockData{Symbol: "AAPL", Price: decimal.NewFromInt(150)}}
	svc := NewInvestmentService(db, market, data.NewPortfolioStore(db), data.NewTradesStore(db))

	executedAt := time.Now()
	mock.ExpectQuery("SELECT id, user_id, symbol").
		WithArgs("user-1", "sell-key-1").
		WillReturnRows(sqlmock.NewRows(idempColsCols).AddRow(
			"trade-sell", "user-1", "AAPL", "SELL", 3, decimal.NewFromInt(150), decimal.NewFromInt(450), executedAt, "COMPLETED",
			"sell-key-1",
		))
	// After replay, GetPortfolioBySymbol returns remaining holding
	mock.ExpectQuery("SELECT id, user_id, symbol").
		WithArgs("user-1", "AAPL").
		WillReturnRows(sqlmock.NewRows(portfolioCols).AddRow(
			"port-1", "user-1", "AAPL", 2, decimal.NewFromInt(150), executedAt, executedAt,
		))

	result, err := svc.SellStock(context.Background(), "user-1", "AAPL", 3, "sell-key-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.Symbol != "AAPL" {
		t.Errorf("expected AAPL stock, got %+v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled sql expectations: %v", err)
	}
}

func TestBuyStock_IdempotencyDifferentArgs(t *testing.T) {
	// Same key, different quantity — should return the original trade result silently.
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	market := &mockMarket{stock: &StockData{Symbol: "AAPL", Price: decimal.NewFromInt(150)}}
	svc := NewInvestmentService(db, market, data.NewPortfolioStore(db), data.NewTradesStore(db))

	executedAt := time.Now()
	// Key found → original trade had qty=5
	mock.ExpectQuery("SELECT id, user_id, symbol").
		WithArgs("user-1", "same-key").
		WillReturnRows(sqlmock.NewRows(idempColsCols).AddRow(
			"trade-original", "user-1", "AAPL", "BUY", 5, decimal.NewFromInt(150), decimal.NewFromInt(750), executedAt, "COMPLETED",
			"same-key",
		))
	mock.ExpectQuery("SELECT id, user_id, symbol").
		WithArgs("user-1", "AAPL").
		WillReturnRows(sqlmock.NewRows(portfolioCols).AddRow(
			"port-1", "user-1", "AAPL", 5, decimal.NewFromInt(150), executedAt, executedAt,
		))

	// Called with qty=10 (different from original 5) — must still replay
	result, err := svc.BuyStock(context.Background(), "user-1", "AAPL", 10, "same-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	// No tx opened
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled sql expectations: %v", err)
	}
}

// TestBuyStock_UniqueViolationButReFetchEmpty exercises the double-rollback path
// (Bug 2): a 23505 unique violation fires on INSERT INTO trades, but the
// post-rollback re-fetch of GetTradeByIdempotencyKey also returns (nil, nil).
// The service must return the original pq.Error, not a nil error with a nil stock.
func TestBuyStock_UniqueViolationButReFetchEmpty(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	market := &mockMarket{stock: &StockData{Symbol: "AAPL", Price: decimal.NewFromInt(100)}}
	svc := NewInvestmentService(db, market, data.NewPortfolioStore(db), data.NewTradesStore(db))

	const ikey = "race-key-xyz"
	pqUniqueViolation := &pq.Error{Code: "23505"}

	// 1. Pre-check: no existing trade for this key.
	mock.ExpectQuery("SELECT id, user_id, symbol").
		WithArgs("user-1", ikey).
		WillReturnRows(sqlmock.NewRows(idempColsCols))

	// 2. Transaction begins.
	mock.ExpectBegin()

	// 3. Balance lock inside tx — user has plenty of balance.
	mock.ExpectQuery("SELECT balance FROM users WHERE id = \\$1 FOR UPDATE").
		WithArgs("user-1").
		WillReturnRows(newBalanceRow(decimal.NewFromFloat(1000.0)))

	// 4. UpdateBalance inside tx.
	mock.ExpectExec("UPDATE users SET balance").
		WithArgs(decimal.NewFromFloat(900.0), "user-1").
		WillReturnResult(sqlmock.NewResult(1, 1))

	// 5. INSERT INTO trades returns a unique-violation error.
	mock.ExpectExec("INSERT INTO trades").
		WillReturnError(pqUniqueViolation)

	// 6. Explicit rollback triggered by the 23505 branch.
	mock.ExpectRollback()

	// 7. Post-rollback re-fetch on s.tradesStore (same underlying db): returns (nil, nil).
	mock.ExpectQuery("SELECT id, user_id, symbol").
		WithArgs("user-1", ikey).
		WillReturnRows(sqlmock.NewRows(idempColsCols))

	stock, err := svc.BuyStock(context.Background(), "user-1", "AAPL", 1, ikey)

	if stock != nil {
		t.Errorf("expected nil stock, got %+v", stock)
	}
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	// The error must be (or wrap) the original pq unique-violation.
	var pqErr *pq.Error
	if !errors.As(err, &pqErr) || pqErr.Code != "23505" {
		t.Errorf("expected pq.Error with code 23505, got %v (%T)", err, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled sql expectations: %v", err)
	}
}
