package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/shopspring/decimal"

	"papertrader/internal/data"
)

// ---- Pure-function tests (no mocks needed) ----

func TestMergePoints_Dedup_FetchedWins(t *testing.T) {
	d := func(s string) time.Time {
		ts, err := time.Parse("2006-01-02", s)
		if err != nil {
			t.Fatalf("parse %q: %v", s, err)
		}
		return ts
	}

	stored := []data.StockHistoryPoint{
		{Symbol: "AAPL", TradeDate: d("2026-01-02"), Close: decimal.NewFromFloat(100), Volume: 10},
		{Symbol: "AAPL", TradeDate: d("2026-01-03"), Close: decimal.NewFromFloat(101), Volume: 11}, // overlaps
	}
	fetched := []data.StockHistoryPoint{
		{Symbol: "AAPL", TradeDate: d("2026-01-03"), Close: decimal.NewFromFloat(999), Volume: 99}, // wins
		{Symbol: "AAPL", TradeDate: d("2026-01-04"), Close: decimal.NewFromFloat(102), Volume: 12},
	}

	merged := mergePoints(stored, fetched)
	if len(merged) != 3 {
		t.Fatalf("merged len: want 3, got %d", len(merged))
	}
	// Verify ordering ASC.
	for i := 1; i < len(merged); i++ {
		if merged[i].TradeDate.Before(merged[i-1].TradeDate) {
			t.Errorf("merged[%d] not after merged[%d]: %v vs %v",
				i, i-1, merged[i].TradeDate, merged[i-1].TradeDate)
		}
	}
	// Conflict winner should be the fetched value (999, not 101).
	for _, p := range merged {
		if p.TradeDate.Equal(d("2026-01-03")) && !p.Close.Equal(decimal.NewFromFloat(999)) {
			t.Errorf("dedup loser: want 999, got %s", p.Close)
		}
	}
}

func TestAssembleSeries_FiltersOutOfWindow(t *testing.T) {
	d := func(s string) time.Time {
		ts, _ := time.Parse("2006-01-02", s)
		return ts
	}

	from := d("2026-01-02")
	to := d("2026-01-04")
	points := []data.StockHistoryPoint{
		{Symbol: "AAPL", TradeDate: d("2026-01-01"), Close: decimal.NewFromFloat(100)}, // before from
		{Symbol: "AAPL", TradeDate: d("2026-01-02"), Close: decimal.NewFromFloat(101)},
		{Symbol: "AAPL", TradeDate: d("2026-01-03"), Close: decimal.NewFromFloat(102)},
		{Symbol: "AAPL", TradeDate: d("2026-01-04"), Close: decimal.NewFromFloat(103)},
		{Symbol: "AAPL", TradeDate: d("2026-01-05"), Close: decimal.NewFromFloat(104)}, // after to
	}

	got := assembleSeries("AAPL", from, to, points)
	if len(got.Points) != 3 {
		t.Fatalf("Points len: want 3, got %d", len(got.Points))
	}
	if got.Symbol != "AAPL" || got.From != "2026-01-02" || got.To != "2026-01-04" {
		t.Errorf("envelope mismatch: %+v", got)
	}
	if got.Points[0].Date != "2026-01-02" || got.Points[2].Date != "2026-01-04" {
		t.Errorf("window edges wrong: %+v", got.Points)
	}
}

// ---- HTTP-mocked tests ----

// fakeHistoricalCache lets us assert MarkRangeEmpty/IsRangeEmpty interactions
// without a Redis dependency.
type fakeHistoricalCache struct {
	mu       sync.Mutex
	emptySet map[string]bool
}

func newFakeHistoricalCache() *fakeHistoricalCache {
	return &fakeHistoricalCache{emptySet: make(map[string]bool)}
}

func (c *fakeHistoricalCache) GetHistorical(_ context.Context, _, _, _ string) (*HistoricalData, error) {
	return nil, nil
}
func (c *fakeHistoricalCache) SetHistorical(_ context.Context, _, _, _ string, _ *HistoricalData, _ time.Duration) error {
	return nil
}
func (c *fakeHistoricalCache) IsRangeEmpty(_ context.Context, symbol, from, to string) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.emptySet[symbol+":"+from+":"+to], nil
}
func (c *fakeHistoricalCache) MarkRangeEmpty(_ context.Context, symbol, from, to string, _ time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.emptySet[symbol+":"+from+":"+to] = true
	return nil
}

// withMockEODServer spins up an httptest.Server that responds with the supplied
// pages keyed by the offset query param. Restores marketStackEODURL on cleanup.
func withMockEODServer(t *testing.T, pages map[string][]marketStackRow, callCount *int) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if callCount != nil {
			*callCount++
		}
		offset := r.URL.Query().Get("offset")
		rows, ok := pages[offset]
		if !ok {
			rows = nil
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(struct {
			Data []marketStackRow `json:"data"`
		}{Data: rows})
	}))
	prev := marketStackEODURL
	marketStackEODURL = srv.URL
	t.Cleanup(func() {
		marketStackEODURL = prev
		srv.Close()
	})
}

type marketStackRow struct {
	Symbol string  `json:"symbol"`
	Date   string  `json:"date"`
	Close  float64 `json:"close"`
	Volume float64 `json:"volume"`
}

func msDate(iso string) string {
	// MarketStack returns "2026-01-02T00:00:00+0000".
	return iso + "T00:00:00+0000"
}

func TestFetchEODSeries_PaginatesUntilShortPage(t *testing.T) {
	page0 := make([]marketStackRow, eodPageSize)
	page1 := make([]marketStackRow, eodPageSize)
	page2 := make([]marketStackRow, 5) // short page → terminates loop

	for i := range page0 {
		page0[i] = marketStackRow{Symbol: "AAPL", Date: msDate("2026-01-02"), Close: 100, Volume: 1}
	}
	for i := range page1 {
		page1[i] = marketStackRow{Symbol: "AAPL", Date: msDate("2026-01-03"), Close: 101, Volume: 1}
	}
	for i := range page2 {
		page2[i] = marketStackRow{Symbol: "AAPL", Date: msDate("2026-01-04"), Close: 102, Volume: 1}
	}

	calls := 0
	withMockEODServer(t, map[string][]marketStackRow{
		"0":   page0,
		"100": page1,
		"200": page2,
	}, &calls)

	svc := &MarketService{apiKey: "test-key"}
	got, err := svc.fetchEODSeries(context.Background(), "AAPL",
		mustDate("2026-01-01"), mustDate("2026-01-10"))
	if err != nil {
		t.Fatalf("fetchEODSeries: %v", err)
	}
	want := eodPageSize*2 + 5
	if len(got) != want {
		t.Errorf("rows: want %d, got %d", want, len(got))
	}
	if calls != 3 {
		t.Errorf("API calls: want 3, got %d", calls)
	}
}

func TestGetHistoricalSeries_ServesFromDB_NoAPICall(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	// The service computes from/to dynamically from time.Now(); generate
	// matching stored rows so the DB fully covers [from, to] and neither
	// gap-fill branch fires.
	const days = 7
	now := time.Now().UTC()
	from := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, -days)
	to := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, -1)

	rows := sqlmock.NewRows([]string{"symbol", "trade_date", "close", "volume"})
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		rows.AddRow("AAPL", d, decimal.NewFromFloat(100), int64(1))
	}
	mock.ExpectQuery("SELECT symbol, trade_date, close, volume").
		WithArgs("AAPL", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(rows)

	calls := 0
	withMockEODServer(t, map[string][]marketStackRow{}, &calls)

	store := data.NewStockHistoryStore(db)
	svc := &MarketService{
		apiKey:            "test-key",
		stockHistoryStore: store,
		historicalCache:   newFakeHistoricalCache(),
	}

	got, err := svc.GetHistoricalSeries(context.Background(), "AAPL", days)
	if err != nil {
		t.Fatalf("GetHistoricalSeries: %v", err)
	}
	if len(got.Points) == 0 {
		t.Errorf("expected points, got empty")
	}
	if calls != 0 {
		t.Errorf("API calls: want 0 (served from DB), got %d", calls)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("sqlmock expectations: %v", err)
	}
}

func TestGetHistoricalSeries_EmptyGapMemoizedToCache(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	// DB is empty for first call → fillGap runs → MarketStack returns empty
	// → InsufficientHistoricalDataError, but the empty marker should be set.
	mock.ExpectQuery("SELECT symbol, trade_date, close, volume").
		WithArgs("AAPL", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"symbol", "trade_date", "close", "volume"}))

	calls := 0
	withMockEODServer(t, map[string][]marketStackRow{}, &calls) // returns empty

	cache := newFakeHistoricalCache()
	svc := &MarketService{
		apiKey:            "test-key",
		stockHistoryStore: data.NewStockHistoryStore(db),
		historicalCache:   cache,
	}

	_, err = svc.GetHistoricalSeries(context.Background(), "AAPL", 30)
	if err == nil {
		t.Fatal("expected InsufficientHistoricalDataError, got nil")
	}
	if calls != 1 {
		t.Errorf("first call should hit MarketStack once, got %d", calls)
	}
	if len(cache.emptySet) == 0 {
		t.Error("expected an empty-range marker in cache, got none")
	}
}

func mustDate(iso string) time.Time {
	ts, err := time.Parse("2006-01-02", iso)
	if err != nil {
		panic(err)
	}
	return ts
}
