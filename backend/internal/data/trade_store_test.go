package data

import (
	"context"
	"database/sql"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/shopspring/decimal"
)

// tradeCols matches the SELECT column list returned by GetTradeByID and
// GetTradesByUserID (total is a computed expression, not a stored column).
var tradeCols = []string{
	"id", "user_id", "symbol", "action", "quantity", "price", "total", "executed_at", "status", "idempotency_key",
}

// ---- CreateTrade ----

func TestCreateTrade_HappyPath(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	trade := &Trade{
		ID:       "trade-1",
		UserID:   "user-1",
		Symbol:   "AAPL",
		Action:   "BUY",
		Quantity: 5,
		Price:    decimal.NewFromFloat(150.0),
		Status:   "COMPLETED",
	}

	mock.ExpectExec("INSERT INTO trades").
		WithArgs(trade.ID, trade.UserID, trade.Symbol, trade.Action, trade.Quantity, trade.Price, trade.Status, sql.NullString{}).
		WillReturnResult(sqlmock.NewResult(1, 1))

	store := NewTradesStore(db)
	if err := store.CreateTrade(context.Background(), trade); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestCreateTrade_DefaultStatus(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	// Status empty → should default to "COMPLETED"
	trade := &Trade{
		ID:       "trade-2",
		UserID:   "user-1",
		Symbol:   "TSLA",
		Action:   "SELL",
		Quantity: 2,
		Price:    decimal.NewFromFloat(250.0),
		Status:   "",
	}

	mock.ExpectExec("INSERT INTO trades").
		WithArgs(trade.ID, trade.UserID, trade.Symbol, trade.Action, trade.Quantity, trade.Price, "COMPLETED", sql.NullString{}).
		WillReturnResult(sqlmock.NewResult(1, 1))

	store := NewTradesStore(db)
	if err := store.CreateTrade(context.Background(), trade); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if trade.Status != "COMPLETED" {
		t.Errorf("Status: got %q, want %q", trade.Status, "COMPLETED")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// ---- GetTradeByID ----

func TestGetTradeByID_Found(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	executedAt := time.Now()
	mock.ExpectQuery("SELECT id, user_id, symbol").
		WithArgs("trade-1").
		WillReturnRows(sqlmock.NewRows(tradeCols).AddRow(
			"trade-1", "user-1", "AAPL", "BUY", 5, decimal.NewFromFloat(150.0), decimal.NewFromFloat(750.0), executedAt, "COMPLETED", nil,
		))

	store := NewTradesStore(db)
	trade, err := store.GetTradeByID(context.Background(), "trade-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if trade.ID != "trade-1" {
		t.Errorf("ID: got %q, want %q", trade.ID, "trade-1")
	}
	if trade.Symbol != "AAPL" {
		t.Errorf("Symbol: got %q, want %q", trade.Symbol, "AAPL")
	}
	if trade.Action != "BUY" {
		t.Errorf("Action: got %q, want %q", trade.Action, "BUY")
	}
	if trade.Quantity != 5 {
		t.Errorf("Quantity: got %d, want %d", trade.Quantity, 5)
	}
	wantTradeTotal := decimal.NewFromFloat(750.0)
	if !trade.Total.Equal(wantTradeTotal) {
		t.Errorf("Total: got %s, want %s", trade.Total, wantTradeTotal)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestGetTradeByID_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("SELECT id, user_id, symbol").
		WithArgs("nonexistent").
		WillReturnRows(sqlmock.NewRows(tradeCols)) // empty → sql.ErrNoRows

	store := NewTradesStore(db)
	_, err = store.GetTradeByID(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for missing trade, got nil")
	}
	if err.Error() != "trade not found" {
		t.Errorf("error: got %q, want %q", err.Error(), "trade not found")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// ---- GetTradesByUserID ----

func TestGetTradesByUserID_HappyPath(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	now := time.Now()
	mock.ExpectQuery(`SELECT id, user_id, symbol, action, quantity, price, \(quantity \* price\) AS total, executed_at, status, idempotency_key`).
		WithArgs("user-1", 50, 0).
		WillReturnRows(sqlmock.NewRows(tradeCols).
			AddRow("t-2", "user-1", "TSLA", "SELL", 3, decimal.NewFromFloat(250.0), decimal.NewFromFloat(750.0), now, "COMPLETED", nil).
			AddRow("t-1", "user-1", "AAPL", "BUY", 5, decimal.NewFromFloat(150.0), decimal.NewFromFloat(750.0), now.Add(-time.Hour), "COMPLETED", nil),
		)

	store := NewTradesStore(db)
	trades, err := store.GetTradesByUserID(context.Background(), "user-1", TradeQueryOpts{Limit: 50, Offset: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(trades) != 2 {
		t.Fatalf("expected 2 trades, got %d", len(trades))
	}
	if trades[0].ID != "t-2" {
		t.Errorf("expected newest trade first, got %q", trades[0].ID)
	}
	wantTradesTotal := decimal.NewFromFloat(750.0)
	if !trades[0].Total.Equal(wantTradesTotal) {
		t.Errorf("Total: got %s, want %s", trades[0].Total, wantTradesTotal)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestGetTradesByUserID_SymbolFilter(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(`AND symbol = \$2`).
		WithArgs("user-1", "AAPL", 50, 0).
		WillReturnRows(sqlmock.NewRows(tradeCols))

	store := NewTradesStore(db)
	if _, err := store.GetTradesByUserID(context.Background(), "user-1", TradeQueryOpts{Symbol: "AAPL", Limit: 50, Offset: 0}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestGetTradesByUserID_SymbolAndActionFilter(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(`AND symbol = \$2 AND action = \$3`).
		WithArgs("user-1", "AAPL", "BUY", 25, 25).
		WillReturnRows(sqlmock.NewRows(tradeCols))

	store := NewTradesStore(db)
	_, err = store.GetTradesByUserID(context.Background(), "user-1", TradeQueryOpts{
		Symbol: "AAPL", Action: "BUY", Limit: 25, Offset: 25,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// ---- CountTradesByUserID ----

func TestCountTradesByUserID_HappyPath(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM trades WHERE user_id = \$1`).
		WithArgs("user-1").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(42))

	store := NewTradesStore(db)
	count, err := store.CountTradesByUserID(context.Background(), "user-1", TradeQueryOpts{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 42 {
		t.Errorf("count: got %d, want 42", count)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestCountTradesByUserID_WithFilter(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM trades WHERE user_id = \$1 AND action = \$2`).
		WithArgs("user-1", "SELL").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(7))

	store := NewTradesStore(db)
	count, err := store.CountTradesByUserID(context.Background(), "user-1", TradeQueryOpts{Action: "SELL"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 7 {
		t.Errorf("count: got %d, want 7", count)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// ---- GetTradeByIdempotencyKey ----

func TestGetTradeByIdempotencyKey_Found(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	now := time.Now()
	ikey := sql.NullString{String: "key-abc", Valid: true}
	mock.ExpectQuery("SELECT id, user_id, symbol").
		WithArgs("user-1", "key-abc").
		WillReturnRows(sqlmock.NewRows(tradeCols).AddRow(
			"trade-1", "user-1", "AAPL", "BUY", 5, decimal.NewFromFloat(150.0), decimal.NewFromFloat(750.0), now, "COMPLETED", ikey,
		))

	store := NewTradesStore(db)
	trade, err := store.GetTradeByIdempotencyKey(context.Background(), "user-1", "key-abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if trade == nil {
		t.Fatal("expected trade, got nil")
	}
	if trade.IdempotencyKey != "key-abc" {
		t.Errorf("IdempotencyKey: got %q, want %q", trade.IdempotencyKey, "key-abc")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestGetTradeByIdempotencyKey_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("SELECT id, user_id, symbol").
		WithArgs("user-1", "missing-key").
		WillReturnRows(sqlmock.NewRows(tradeCols))

	store := NewTradesStore(db)
	trade, err := store.GetTradeByIdempotencyKey(context.Background(), "user-1", "missing-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if trade != nil {
		t.Errorf("expected nil trade for missing key, got %+v", trade)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
