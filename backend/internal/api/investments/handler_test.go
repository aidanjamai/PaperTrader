package investments

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shopspring/decimal"

	"papertrader/internal/data"
	"papertrader/internal/service"
)

// mockInvestmentService implements InvestmentServicer for handler tests.
type mockInvestmentService struct {
	buyResult          *data.UserStock
	buyErr             error
	sellResult         *data.UserStock
	sellErr            error
	stocks             []data.UserStock
	stocksErr          error
	trades             []data.Trade
	tradesTotal        int
	tradesErr          error
	lastTradeOpts      data.TradeQueryOpts
	lastIdempotencyKey string
}

func (m *mockInvestmentService) BuyStock(_ context.Context, userID, symbol string, quantity int, idempotencyKey string) (*data.UserStock, error) {
	m.lastIdempotencyKey = idempotencyKey
	return m.buyResult, m.buyErr
}
func (m *mockInvestmentService) SellStock(_ context.Context, userID, symbol string, quantity int, idempotencyKey string) (*data.UserStock, error) {
	m.lastIdempotencyKey = idempotencyKey
	return m.sellResult, m.sellErr
}
func (m *mockInvestmentService) GetUserStocks(_ context.Context, userID string) ([]data.UserStock, error) {
	return m.stocks, m.stocksErr
}
func (m *mockInvestmentService) GetUserTrades(_ context.Context, userID string, opts data.TradeQueryOpts) ([]data.Trade, int, error) {
	m.lastTradeOpts = opts
	return m.trades, m.tradesTotal, m.tradesErr
}

func newHandler(svc InvestmentServicer) *InvestmentsHandler {
	return &InvestmentsHandler{service: svc}
}

func jsonReq(t *testing.T, method, target string, body interface{}) *http.Request {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	return httptest.NewRequest(method, target, bytes.NewReader(b))
}

// ---- BuyStock ----

func TestBuyStock_MissingUserID(t *testing.T) {
	h := newHandler(&mockInvestmentService{})
	req := jsonReq(t, http.MethodPost, "/buy", BuyStockRequest{Symbol: "AAPL", Quantity: 1})
	w := httptest.NewRecorder()
	h.BuyStock(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestBuyStock_InvalidQuantity(t *testing.T) {
	h := newHandler(&mockInvestmentService{})
	req := jsonReq(t, http.MethodPost, "/buy", BuyStockRequest{Symbol: "AAPL", Quantity: 0})
	req.Header.Set("X-User-ID", "user-1")
	w := httptest.NewRecorder()
	h.BuyStock(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestBuyStock_InvalidSymbol(t *testing.T) {
	h := newHandler(&mockInvestmentService{})
	// symbol starts with digits — invalid
	req := jsonReq(t, http.MethodPost, "/buy", BuyStockRequest{Symbol: "123BAD", Quantity: 1})
	req.Header.Set("X-User-ID", "user-1")
	w := httptest.NewRecorder()
	h.BuyStock(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestBuyStock_InsufficientFunds(t *testing.T) {
	h := newHandler(&mockInvestmentService{buyErr: &service.InsufficientFundsError{}})
	req := jsonReq(t, http.MethodPost, "/buy", BuyStockRequest{Symbol: "AAPL", Quantity: 1})
	req.Header.Set("X-User-ID", "user-1")
	w := httptest.NewRecorder()
	h.BuyStock(w, req)
	// MapServiceError dispatches on the typed HTTPError, returning 400.
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestBuyStock_Success(t *testing.T) {
	stock := &data.UserStock{ID: "port-1", UserID: "user-1", Symbol: "AAPL", Quantity: 5}
	h := newHandler(&mockInvestmentService{buyResult: stock})
	req := jsonReq(t, http.MethodPost, "/buy", BuyStockRequest{Symbol: "AAPL", Quantity: 5})
	req.Header.Set("X-User-ID", "user-1")
	w := httptest.NewRecorder()
	h.BuyStock(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var result data.UserStock
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if result.Symbol != "AAPL" {
		t.Errorf("symbol: got %q, want %q", result.Symbol, "AAPL")
	}
}

// ---- SellStock ----

func TestSellStock_MissingUserID(t *testing.T) {
	h := newHandler(&mockInvestmentService{})
	req := jsonReq(t, http.MethodPost, "/sell", SellStockRequest{Symbol: "AAPL", Quantity: 1})
	w := httptest.NewRecorder()
	h.SellStock(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestSellStock_InvalidQuantity(t *testing.T) {
	h := newHandler(&mockInvestmentService{})
	req := jsonReq(t, http.MethodPost, "/sell", SellStockRequest{Symbol: "AAPL", Quantity: -1})
	req.Header.Set("X-User-ID", "user-1")
	w := httptest.NewRecorder()
	h.SellStock(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestSellStock_InvalidSymbol(t *testing.T) {
	h := newHandler(&mockInvestmentService{})
	req := jsonReq(t, http.MethodPost, "/sell", SellStockRequest{Symbol: "", Quantity: 1})
	req.Header.Set("X-User-ID", "user-1")
	w := httptest.NewRecorder()
	h.SellStock(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestSellStock_HoldingNotFound(t *testing.T) {
	h := newHandler(&mockInvestmentService{sellErr: &service.StockHoldingNotFoundError{}})
	req := jsonReq(t, http.MethodPost, "/sell", SellStockRequest{Symbol: "TSLA", Quantity: 1})
	req.Header.Set("X-User-ID", "user-1")
	w := httptest.NewRecorder()
	h.SellStock(w, req)
	// MapServiceError dispatches on the typed HTTPError, returning 404.
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestSellStock_Success(t *testing.T) {
	stock := &data.UserStock{ID: "port-1", UserID: "user-1", Symbol: "AAPL", Quantity: 3}
	h := newHandler(&mockInvestmentService{sellResult: stock})
	req := jsonReq(t, http.MethodPost, "/sell", SellStockRequest{Symbol: "AAPL", Quantity: 2})
	req.Header.Set("X-User-ID", "user-1")
	w := httptest.NewRecorder()
	h.SellStock(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var result data.UserStock
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if result.Symbol != "AAPL" {
		t.Errorf("symbol: got %q, want %q", result.Symbol, "AAPL")
	}
}

// ---- GetUserStocks ----

func TestGetUserStocks_MissingUserID(t *testing.T) {
	h := newHandler(&mockInvestmentService{})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.GetUserStocks(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestGetUserStocks_Success(t *testing.T) {
	stocks := []data.UserStock{
		{ID: "p1", UserID: "user-1", Symbol: "AAPL", Quantity: 5},
		{ID: "p2", UserID: "user-1", Symbol: "TSLA", Quantity: 2},
	}
	h := newHandler(&mockInvestmentService{stocks: stocks})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-User-ID", "user-1")
	w := httptest.NewRecorder()
	h.GetUserStocks(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var result []data.UserStock
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 stocks, got %d", len(result))
	}
}

func TestGetUserStocks_Empty(t *testing.T) {
	h := newHandler(&mockInvestmentService{stocks: []data.UserStock{}})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-User-ID", "user-1")
	w := httptest.NewRecorder()
	h.GetUserStocks(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for empty portfolio, got %d", w.Code)
	}
}

// ---- GetTradeHistory ----

func TestGetTradeHistory_MissingUserID(t *testing.T) {
	h := newHandler(&mockInvestmentService{})
	req := httptest.NewRequest(http.MethodGet, "/history", nil)
	w := httptest.NewRecorder()
	h.GetTradeHistory(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestGetTradeHistory_InvalidLimit(t *testing.T) {
	h := newHandler(&mockInvestmentService{})
	req := httptest.NewRequest(http.MethodGet, "/history?limit=999", nil)
	req.Header.Set("X-User-ID", "user-1")
	w := httptest.NewRecorder()
	h.GetTradeHistory(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for limit > 200, got %d", w.Code)
	}
}

func TestGetTradeHistory_InvalidOffset(t *testing.T) {
	h := newHandler(&mockInvestmentService{})
	req := httptest.NewRequest(http.MethodGet, "/history?offset=-1", nil)
	req.Header.Set("X-User-ID", "user-1")
	w := httptest.NewRecorder()
	h.GetTradeHistory(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for negative offset, got %d", w.Code)
	}
}

func TestGetTradeHistory_InvalidAction(t *testing.T) {
	h := newHandler(&mockInvestmentService{})
	req := httptest.NewRequest(http.MethodGet, "/history?action=HOLD", nil)
	req.Header.Set("X-User-ID", "user-1")
	w := httptest.NewRecorder()
	h.GetTradeHistory(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid action, got %d", w.Code)
	}
}

func TestGetTradeHistory_InvalidSymbol(t *testing.T) {
	h := newHandler(&mockInvestmentService{})
	req := httptest.NewRequest(http.MethodGet, "/history?symbol=123BAD", nil)
	req.Header.Set("X-User-ID", "user-1")
	w := httptest.NewRecorder()
	h.GetTradeHistory(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid symbol, got %d", w.Code)
	}
}

func TestGetTradeHistory_DefaultsApplied(t *testing.T) {
	mock := &mockInvestmentService{trades: []data.Trade{}, tradesTotal: 0}
	h := newHandler(mock)
	req := httptest.NewRequest(http.MethodGet, "/history", nil)
	req.Header.Set("X-User-ID", "user-1")
	w := httptest.NewRecorder()
	h.GetTradeHistory(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if mock.lastTradeOpts.Limit != 50 {
		t.Errorf("default limit: got %d, want 50", mock.lastTradeOpts.Limit)
	}
	if mock.lastTradeOpts.Offset != 0 {
		t.Errorf("default offset: got %d, want 0", mock.lastTradeOpts.Offset)
	}
}

func TestGetTradeHistory_Success(t *testing.T) {
	trades := []data.Trade{
		{ID: "t1", UserID: "user-1", Symbol: "AAPL", Action: "BUY", Quantity: 5, Price: decimal.NewFromInt(150), Total: decimal.NewFromInt(750), Status: "COMPLETED"},
		{ID: "t2", UserID: "user-1", Symbol: "TSLA", Action: "SELL", Quantity: 2, Price: decimal.NewFromInt(250), Total: decimal.NewFromInt(500), Status: "COMPLETED"},
	}
	mock := &mockInvestmentService{trades: trades, tradesTotal: 2}
	h := newHandler(mock)
	req := httptest.NewRequest(http.MethodGet, "/history?limit=10&offset=0&symbol=AAPL&action=BUY", nil)
	req.Header.Set("X-User-ID", "user-1")
	w := httptest.NewRecorder()
	h.GetTradeHistory(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp TradeHistoryResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(resp.Trades) != 2 {
		t.Errorf("trades: got %d, want 2", len(resp.Trades))
	}
	if resp.Total != 2 {
		t.Errorf("total: got %d, want 2", resp.Total)
	}
	if resp.Limit != 10 {
		t.Errorf("limit: got %d, want 10", resp.Limit)
	}
	if mock.lastTradeOpts.Symbol != "AAPL" {
		t.Errorf("symbol: got %q, want AAPL", mock.lastTradeOpts.Symbol)
	}
	if mock.lastTradeOpts.Action != "BUY" {
		t.Errorf("action: got %q, want BUY", mock.lastTradeOpts.Action)
	}
}

// ---- Idempotency-Key header tests ----

func TestBuyStock_HeaderPropagated(t *testing.T) {
	stock := &data.UserStock{ID: "port-1", UserID: "user-1", Symbol: "AAPL", Quantity: 5}
	mock := &mockInvestmentService{buyResult: stock}
	h := newHandler(mock)

	req := jsonReq(t, http.MethodPost, "/buy", BuyStockRequest{Symbol: "AAPL", Quantity: 5})
	req.Header.Set("X-User-ID", "user-1")
	req.Header.Set("Idempotency-Key", "test-key-123")
	w := httptest.NewRecorder()
	h.BuyStock(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if mock.lastIdempotencyKey != "test-key-123" {
		t.Errorf("idempotencyKey: got %q, want %q", mock.lastIdempotencyKey, "test-key-123")
	}
}

func TestBuyStock_RejectsInvalidIdempotencyKey_TooLong(t *testing.T) {
	h := newHandler(&mockInvestmentService{})

	// build the key simply:
	key256 := ""
	for i := 0; i < 256; i++ {
		key256 += "a"
	}

	req := jsonReq(t, http.MethodPost, "/buy", BuyStockRequest{Symbol: "AAPL", Quantity: 1})
	req.Header.Set("X-User-ID", "user-1")
	req.Header.Set("Idempotency-Key", key256)
	w := httptest.NewRecorder()
	h.BuyStock(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for too-long key, got %d", w.Code)
	}
}

func TestBuyStock_RejectsInvalidIdempotencyKey_NonASCII(t *testing.T) {
	h := newHandler(&mockInvestmentService{})

	req := jsonReq(t, http.MethodPost, "/buy", BuyStockRequest{Symbol: "AAPL", Quantity: 1})
	req.Header.Set("X-User-ID", "user-1")
	// Tab character (0x09) is below 0x20 — not printable ASCII.
	req.Header.Set("Idempotency-Key", "key\twith\ttabs")
	w := httptest.NewRecorder()
	h.BuyStock(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for non-printable-ASCII key, got %d", w.Code)
	}
}

func TestSellStock_HeaderPropagated(t *testing.T) {
	stock := &data.UserStock{ID: "port-1", UserID: "user-1", Symbol: "AAPL", Quantity: 3}
	mock := &mockInvestmentService{sellResult: stock}
	h := newHandler(mock)

	req := jsonReq(t, http.MethodPost, "/sell", SellStockRequest{Symbol: "AAPL", Quantity: 2})
	req.Header.Set("X-User-ID", "user-1")
	req.Header.Set("Idempotency-Key", "sell-key-456")
	w := httptest.NewRecorder()
	h.SellStock(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if mock.lastIdempotencyKey != "sell-key-456" {
		t.Errorf("idempotencyKey: got %q, want %q", mock.lastIdempotencyKey, "sell-key-456")
	}
}

func TestBuyStock_RejectsBlankIdempotencyKey(t *testing.T) {
	h := newHandler(&mockInvestmentService{})

	req := jsonReq(t, http.MethodPost, "/buy", BuyStockRequest{Symbol: "AAPL", Quantity: 1})
	req.Header.Set("X-User-ID", "user-1")
	req.Header.Set("Idempotency-Key", "   ")
	w := httptest.NewRecorder()
	h.BuyStock(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for blank idempotency key, got %d", w.Code)
	}
}
