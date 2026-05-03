package data

import (
	"context"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/shopspring/decimal"
)

// portfolioQueryCols matches the SELECT column list in GetPortfolioByUserID / GetPortfolioBySymbol.
var portfolioQueryCols = []string{
	"id", "user_id", "symbol", "quantity", "avg_price", "created_at", "updated_at",
}

func portfolioRow(id, userID, symbol string, quantity int, avgPrice decimal.Decimal) *sqlmock.Rows {
	return sqlmock.NewRows(portfolioQueryCols).AddRow(
		id, userID, symbol, quantity, avgPrice, time.Now(), time.Now(),
	)
}

// ---- GetPortfolioByUserID ----

func TestGetPortfolioByUserID_ReturnsHoldings(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows(portfolioQueryCols).
		AddRow("p1", "user-1", "AAPL", 10, decimal.NewFromFloat(150.0), time.Now(), time.Now()).
		AddRow("p2", "user-1", "TSLA", 5, decimal.NewFromFloat(250.0), time.Now(), time.Now())

	mock.ExpectQuery("SELECT id, user_id, symbol").
		WithArgs("user-1").
		WillReturnRows(rows)

	store := NewPortfolioStore(db)
	holdings, err := store.GetPortfolioByUserID(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(holdings) != 2 {
		t.Errorf("expected 2 holdings, got %d", len(holdings))
	}
	if holdings[0].Symbol != "AAPL" {
		t.Errorf("first symbol: got %q, want %q", holdings[0].Symbol, "AAPL")
	}
	// Total should be computed: avgPrice * quantity
	wantTotal := decimal.NewFromFloat(150.0 * 10)
	if !holdings[0].Total.Equal(wantTotal) {
		t.Errorf("Total: got %s, want %s", holdings[0].Total, wantTotal)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestGetPortfolioByUserID_Empty(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("SELECT id, user_id, symbol").
		WithArgs("user-99").
		WillReturnRows(sqlmock.NewRows(portfolioQueryCols))

	store := NewPortfolioStore(db)
	holdings, err := store.GetPortfolioByUserID(context.Background(), "user-99")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// nil slice is acceptable for an empty portfolio
	if len(holdings) != 0 {
		t.Errorf("expected 0 holdings, got %d", len(holdings))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// ---- GetPortfolioBySymbol ----

func TestGetPortfolioBySymbol_Found(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("SELECT id, user_id, symbol").
		WithArgs("user-1", "AAPL").
		WillReturnRows(portfolioRow("p1", "user-1", "AAPL", 10, decimal.NewFromFloat(150.0)))

	store := NewPortfolioStore(db)
	holding, err := store.GetPortfolioBySymbol(context.Background(), "user-1", "AAPL")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if holding.Symbol != "AAPL" {
		t.Errorf("Symbol: got %q, want %q", holding.Symbol, "AAPL")
	}
	if holding.Quantity != 10 {
		t.Errorf("Quantity: got %d, want %d", holding.Quantity, 10)
	}
	wantHoldingTotal := decimal.NewFromFloat(150.0 * 10)
	if !holding.Total.Equal(wantHoldingTotal) {
		t.Errorf("Total: got %s, want %s", holding.Total, wantHoldingTotal)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestGetPortfolioBySymbol_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("SELECT id, user_id, symbol").
		WithArgs("user-1", "MSFT").
		WillReturnRows(sqlmock.NewRows(portfolioQueryCols)) // empty → ErrNoRows

	store := NewPortfolioStore(db)
	_, err = store.GetPortfolioBySymbol(context.Background(), "user-1", "MSFT")
	if err == nil {
		t.Fatal("expected error for missing holding, got nil")
	}
	if err != ErrStockHoldingNotFound {
		t.Errorf("error: got %v, want ErrStockHoldingNotFound", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// ---- UpdatePortfolioWithBuy (upsert, new holding) ----

func TestUpdatePortfolioWithBuy_NewHolding(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	// 1. GetPortfolioBySymbol → no rows (new holding)
	mock.ExpectQuery("SELECT id, user_id, symbol").
		WithArgs("user-1", "NVDA").
		WillReturnRows(sqlmock.NewRows(portfolioQueryCols))

	// 2. INSERT ... ON CONFLICT upsert
	mock.ExpectExec("INSERT INTO portfolio").
		WithArgs(sqlmock.AnyArg(), "user-1", "NVDA", 3, decimal.NewFromFloat(500.0)).
		WillReturnResult(sqlmock.NewResult(1, 1))

	store := NewPortfolioStore(db)
	if err := store.UpdatePortfolioWithBuy(context.Background(), "user-1", "NVDA", 3, decimal.NewFromFloat(500.0)); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestUpdatePortfolioWithBuy_ExistingHolding(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	// 1. GetPortfolioBySymbol → existing row (5 shares @ $100)
	mock.ExpectQuery("SELECT id, user_id, symbol").
		WithArgs("user-1", "AAPL").
		WillReturnRows(portfolioRow("p1", "user-1", "AAPL", 5, decimal.NewFromFloat(100.0)))

	// 2. Upsert: new qty=5+3=8, new avg=(100*5 + 200*3)/8 = 1100/8 = 137.5
	mock.ExpectExec("INSERT INTO portfolio").
		WithArgs("p1", "user-1", "AAPL", 8, decimal.NewFromFloat(137.5)).
		WillReturnResult(sqlmock.NewResult(1, 1))

	store := NewPortfolioStore(db)
	if err := store.UpdatePortfolioWithBuy(context.Background(), "user-1", "AAPL", 3, decimal.NewFromFloat(200.0)); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
