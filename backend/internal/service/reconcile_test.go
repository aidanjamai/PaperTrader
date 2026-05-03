package service

import (
	"context"
	"database/sql"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/shopspring/decimal"

	"papertrader/internal/data"
)

// allTradesCols matches GetAllTradesByUserID SELECT list.
var allTradesCols = []string{
	"id", "user_id", "symbol", "action", "quantity", "price", "total", "executed_at", "status", "idempotency_key",
}

// portfolioRowCols matches GetPortfolioByUserID SELECT list.
var portfolioRowCols = []string{
	"id", "user_id", "symbol", "quantity", "avg_price", "created_at", "updated_at",
}

func newReconcileService(db *sql.DB) *ReconcileService {
	return NewReconcileService(db, data.NewPortfolioStore(db), data.NewTradesStore(db))
}

// addTrade is a helper to add a trade row to sqlmock rows.
func addTrade(rows *sqlmock.Rows, id, userID, symbol, action string, qty int, price decimal.Decimal, at time.Time) *sqlmock.Rows {
	total := price.Mul(decimal.NewFromInt(int64(qty)))
	return rows.AddRow(id, userID, symbol, action, qty, price, total, at, "COMPLETED", nil)
}

// ---- TestReconcile_NoDiscrepanciesAfterTrades ----

func TestReconcile_NoDiscrepanciesAfterTrades(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	svc := newReconcileService(db)
	now := time.Now()

	// 3 BUYs + 1 SELL: net 4 shares at avg (100*2 + 110*3) / 5 → then sell 1 → 4 shares
	// BUY 2@100, BUY 3@110 → avg = (200+330)/5 = 106, qty=5
	// SELL 1 → qty=4, avg=106
	tradeRows := sqlmock.NewRows(allTradesCols)
	addTrade(tradeRows, "t1", "user-1", "AAPL", "BUY", 2, decimal.NewFromFloat(100.0), now.Add(-4*time.Hour))
	addTrade(tradeRows, "t2", "user-1", "AAPL", "BUY", 3, decimal.NewFromFloat(110.0), now.Add(-3*time.Hour))
	addTrade(tradeRows, "t3", "user-1", "TSLA", "BUY", 1, decimal.NewFromFloat(200.0), now.Add(-2*time.Hour))
	addTrade(tradeRows, "t4", "user-1", "AAPL", "SELL", 1, decimal.NewFromFloat(105.0), now.Add(-1*time.Hour))

	mock.ExpectQuery("SELECT id, user_id, symbol").
		WithArgs("user-1").
		WillReturnRows(tradeRows)

	// Expected: AAPL qty=4, avg=106; TSLA qty=1, avg=200
	portRows := sqlmock.NewRows(portfolioRowCols).
		AddRow("p1", "user-1", "AAPL", 4, decimal.NewFromFloat(106.0), now, now).
		AddRow("p2", "user-1", "TSLA", 1, decimal.NewFromFloat(200.0), now, now)
	mock.ExpectQuery("SELECT id, user_id, symbol").
		WithArgs("user-1").
		WillReturnRows(portRows)

	discrepancies, err := svc.Reconcile(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(discrepancies) != 0 {
		t.Errorf("expected no discrepancies, got %+v", discrepancies)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled sql expectations: %v", err)
	}
}

// ---- TestReconcile_DetectsQuantityMismatch ----

func TestReconcile_DetectsQuantityMismatch(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	svc := newReconcileService(db)
	now := time.Now()

	// Ledger: BUY 7 AAPL
	tradeRows := sqlmock.NewRows(allTradesCols)
	addTrade(tradeRows, "t1", "user-1", "AAPL", "BUY", 7, decimal.NewFromFloat(100.0), now)

	mock.ExpectQuery("SELECT id, user_id, symbol").
		WithArgs("user-1").
		WillReturnRows(tradeRows)

	// Portfolio: shows only 5 (drift)
	portRows := sqlmock.NewRows(portfolioRowCols).
		AddRow("p1", "user-1", "AAPL", 5, decimal.NewFromFloat(100.0), now, now)
	mock.ExpectQuery("SELECT id, user_id, symbol").
		WithArgs("user-1").
		WillReturnRows(portRows)

	discrepancies, err := svc.Reconcile(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(discrepancies) != 1 {
		t.Fatalf("expected 1 discrepancy, got %d: %+v", len(discrepancies), discrepancies)
	}
	if discrepancies[0].Kind != KindQuantityMismatch {
		t.Errorf("kind: got %q, want %q", discrepancies[0].Kind, KindQuantityMismatch)
	}
	if discrepancies[0].LedgerQty != 7 {
		t.Errorf("ledger qty: got %d, want 7", discrepancies[0].LedgerQty)
	}
	if discrepancies[0].PortfolioQty != 5 {
		t.Errorf("portfolio qty: got %d, want 5", discrepancies[0].PortfolioQty)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled sql expectations: %v", err)
	}
}

// ---- TestReconcile_DetectsAvgPriceMismatch ----

func TestReconcile_DetectsAvgPriceMismatch(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	svc := newReconcileService(db)
	now := time.Now()

	// Ledger: BUY 5@100 → avg=100
	tradeRows := sqlmock.NewRows(allTradesCols)
	addTrade(tradeRows, "t1", "user-1", "AAPL", "BUY", 5, decimal.NewFromFloat(100.0), now)

	mock.ExpectQuery("SELECT id, user_id, symbol").
		WithArgs("user-1").
		WillReturnRows(tradeRows)

	// Portfolio: same qty but wrong avg (150)
	portRows := sqlmock.NewRows(portfolioRowCols).
		AddRow("p1", "user-1", "AAPL", 5, decimal.NewFromFloat(150.0), now, now)
	mock.ExpectQuery("SELECT id, user_id, symbol").
		WithArgs("user-1").
		WillReturnRows(portRows)

	discrepancies, err := svc.Reconcile(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(discrepancies) != 1 {
		t.Fatalf("expected 1 discrepancy, got %d: %+v", len(discrepancies), discrepancies)
	}
	if discrepancies[0].Kind != KindAvgPriceMismatch {
		t.Errorf("kind: got %q, want %q", discrepancies[0].Kind, KindAvgPriceMismatch)
	}
	wantLedgerAvg := decimal.NewFromFloat(100.0)
	if !discrepancies[0].LedgerAvg.Equal(wantLedgerAvg) {
		t.Errorf("ledger avg: got %s, want 100", discrepancies[0].LedgerAvg)
	}
	wantPortfolioAvg := decimal.NewFromFloat(150.0)
	if !discrepancies[0].PortfolioAvg.Equal(wantPortfolioAvg) {
		t.Errorf("portfolio avg: got %s, want 150", discrepancies[0].PortfolioAvg)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled sql expectations: %v", err)
	}
}

// ---- TestReconcile_DetectsOrphanPortfolio ----

func TestReconcile_DetectsOrphanPortfolio(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	svc := newReconcileService(db)
	now := time.Now()

	// Ledger: no AAPL trades at all
	mock.ExpectQuery("SELECT id, user_id, symbol").
		WithArgs("user-1").
		WillReturnRows(sqlmock.NewRows(allTradesCols))

	// Portfolio: has AAPL (orphan)
	portRows := sqlmock.NewRows(portfolioRowCols).
		AddRow("p1", "user-1", "AAPL", 3, decimal.NewFromFloat(100.0), now, now)
	mock.ExpectQuery("SELECT id, user_id, symbol").
		WithArgs("user-1").
		WillReturnRows(portRows)

	discrepancies, err := svc.Reconcile(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(discrepancies) != 1 {
		t.Fatalf("expected 1 discrepancy, got %d: %+v", len(discrepancies), discrepancies)
	}
	if discrepancies[0].Kind != KindOrphanPortfolio {
		t.Errorf("kind: got %q, want %q", discrepancies[0].Kind, KindOrphanPortfolio)
	}
	if discrepancies[0].Symbol != "AAPL" {
		t.Errorf("symbol: got %q, want AAPL", discrepancies[0].Symbol)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled sql expectations: %v", err)
	}
}

// ---- TestReconcile_DetectsNegativeLedgerQty ----

func TestReconcile_DetectsNegativeLedgerQty(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	svc := newReconcileService(db)
	now := time.Now()

	// Ledger: BUY 2 AAPL then SELL 5 AAPL — net -3, a real anomaly.
	tradeRows := sqlmock.NewRows(allTradesCols)
	addTrade(tradeRows, "t1", "user-1", "AAPL", "BUY", 2, decimal.NewFromFloat(100.0), now.Add(-2*time.Hour))
	addTrade(tradeRows, "t2", "user-1", "AAPL", "SELL", 5, decimal.NewFromFloat(110.0), now.Add(-1*time.Hour))

	mock.ExpectQuery("SELECT id, user_id, symbol").
		WithArgs("user-1").
		WillReturnRows(tradeRows)

	// Portfolio: empty (makes sense — you can't hold negative shares).
	mock.ExpectQuery("SELECT id, user_id, symbol").
		WithArgs("user-1").
		WillReturnRows(sqlmock.NewRows(portfolioRowCols))

	discrepancies, err := svc.Reconcile(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(discrepancies) != 1 {
		t.Fatalf("expected 1 discrepancy, got %d: %+v", len(discrepancies), discrepancies)
	}
	d := discrepancies[0]
	if d.Kind != KindNegativeLedgerQty {
		t.Errorf("kind: got %q, want %q", d.Kind, KindNegativeLedgerQty)
	}
	if d.LedgerQty != -3 {
		t.Errorf("LedgerQty: got %d, want -3", d.LedgerQty)
	}
	if d.Symbol != "AAPL" {
		t.Errorf("symbol: got %q, want AAPL", d.Symbol)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled sql expectations: %v", err)
	}
}

// ---- TestReconcile_DetectsMissingPortfolio ----

func TestReconcile_DetectsMissingPortfolio(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	svc := newReconcileService(db)
	now := time.Now()

	// Ledger: BUY 5 AAPL → net positive position
	tradeRows := sqlmock.NewRows(allTradesCols)
	addTrade(tradeRows, "t1", "user-1", "AAPL", "BUY", 5, decimal.NewFromFloat(100.0), now)

	mock.ExpectQuery("SELECT id, user_id, symbol").
		WithArgs("user-1").
		WillReturnRows(tradeRows)

	// Portfolio: empty (AAPL row missing)
	mock.ExpectQuery("SELECT id, user_id, symbol").
		WithArgs("user-1").
		WillReturnRows(sqlmock.NewRows(portfolioRowCols))

	discrepancies, err := svc.Reconcile(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(discrepancies) != 1 {
		t.Fatalf("expected 1 discrepancy, got %d: %+v", len(discrepancies), discrepancies)
	}
	if discrepancies[0].Kind != KindMissingPortfolio {
		t.Errorf("kind: got %q, want %q", discrepancies[0].Kind, KindMissingPortfolio)
	}
	if discrepancies[0].LedgerQty != 5 {
		t.Errorf("ledger qty: got %d, want 5", discrepancies[0].LedgerQty)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled sql expectations: %v", err)
	}
}
