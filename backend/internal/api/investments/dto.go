package investments

import (
	"papertrader/internal/data"
)

// BuyStockRequest / SellStockRequest are decoded from the JSON body of the
// /buy and /sell endpoints. The optional `userId` field is accepted for
// backwards compatibility but ignored; the authoritative user is whatever the
// JWT middleware writes into X-User-ID.
type BuyStockRequest struct {
	UserID   string `json:"userId"`
	Symbol   string `json:"symbol"`
	Quantity int    `json:"quantity"`
}

type SellStockRequest struct {
	UserID   string `json:"userId"`
	Symbol   string `json:"symbol"`
	Quantity int    `json:"quantity"`
}

// TradeHistoryResponse is the paginated payload returned by GET /investments/history.
// Total is the count of all trades matching the filter (independent of limit/offset),
// so the UI can render "showing 1-50 of 142" and decide whether Next is enabled.
type TradeHistoryResponse struct {
	Trades []data.Trade `json:"trades"`
	Total  int          `json:"total"`
	Limit  int          `json:"limit"`
	Offset int          `json:"offset"`
}
