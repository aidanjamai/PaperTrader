package data

import "context"

// Trades is the storage contract for the trades append-only log.
// Implemented by TradesStore (see trade.go).
// Mutations (UPDATE/DELETE) are intentionally absent — the DB enforces
// append-only semantics via triggers on the trades table.
type Trades interface {
	CreateTrade(ctx context.Context, trade *Trade) error
	GetTradeByID(ctx context.Context, id string) (*Trade, error)
	GetTradesByUserID(ctx context.Context, userID string, opts TradeQueryOpts) ([]Trade, error)
	CountTradesByUserID(ctx context.Context, userID string, opts TradeQueryOpts) (int, error)
	GetAllTradesByUserID(ctx context.Context, userID string) ([]Trade, error)
	GetTradeByIdempotencyKey(ctx context.Context, userID, key string) (*Trade, error)
}
