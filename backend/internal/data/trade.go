package data

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"time"

	"github.com/shopspring/decimal"
)

// Trade is an append-only log entry for a buy/sell. ExecutedAt is set by the DB
// default; Total is computed in SELECT (quantity * price), never stored.
type Trade struct {
	ID             string          `json:"id"`
	UserID         string          `json:"user_id"`
	Symbol         string          `json:"symbol"`
	Action         string          `json:"action"`
	Quantity       int             `json:"quantity"`
	Price          decimal.Decimal `json:"price"`
	Total          decimal.Decimal `json:"total"`
	ExecutedAt     time.Time       `json:"executed_at"`
	Status         string          `json:"status"` // PENDING, COMPLETED, FAILED
	IdempotencyKey string          `json:"idempotency_key,omitempty"`
}

// TradeQueryOpts are filters/pagination for GetTradesByUserID and CountTradesByUserID.
// Symbol and Action are optional ("" means no filter); Limit/Offset are pre-validated by the handler.
type TradeQueryOpts struct {
	Symbol string
	Action string
	Limit  int
	Offset int
}

type TradesStore struct {
	db DBTX
}

func NewTradesStore(db DBTX) *TradesStore {
	return &TradesStore{db: db}
}

// CreateTrade inserts a trade row. ExecutedAt is left to the DB default
// (CURRENT_TIMESTAMP) so callers don't need to set it.
// IdempotencyKey is stored as NULL when empty so the partial unique index
// only constrains non-null keys (legacy rows are unaffected).
func (uts *TradesStore) CreateTrade(ctx context.Context, trade *Trade) error {
	if trade.Status == "" {
		trade.Status = "COMPLETED"
	}
	var ikey sql.NullString
	if trade.IdempotencyKey != "" {
		ikey = sql.NullString{String: trade.IdempotencyKey, Valid: true}
	}
	query := `INSERT INTO trades (id, user_id, symbol, action, quantity, price, status, idempotency_key) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
	_, err := uts.db.ExecContext(ctx, query, trade.ID, trade.UserID, trade.Symbol, trade.Action, trade.Quantity, trade.Price, trade.Status, ikey)
	return err
}

func (uts *TradesStore) GetTradeByID(ctx context.Context, id string) (*Trade, error) {
	query := `SELECT id, user_id, symbol, action, quantity, price, (quantity * price) AS total, executed_at, status, idempotency_key FROM trades WHERE id = $1`

	var trade Trade
	var ikey sql.NullString
	err := uts.db.QueryRowContext(ctx, query, id).Scan(&trade.ID, &trade.UserID, &trade.Symbol, &trade.Action, &trade.Quantity, &trade.Price, &trade.Total, &trade.ExecutedAt, &trade.Status, &ikey)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("trade not found")
		}
		return nil, err
	}
	if ikey.Valid {
		trade.IdempotencyKey = ikey.String
	}

	return &trade, nil
}

// buildTradeFilter assembles the optional WHERE clauses shared by
// GetTradesByUserID and CountTradesByUserID so the two stay in sync.
// startIdx is the next placeholder number to use ($1 is reserved for userID).
func buildTradeFilter(opts TradeQueryOpts, startIdx int) (string, []interface{}) {
	clauses := ""
	args := []interface{}{}
	idx := startIdx
	if opts.Symbol != "" {
		clauses += " AND symbol = $" + strconv.Itoa(idx)
		args = append(args, opts.Symbol)
		idx++
	}
	if opts.Action != "" {
		clauses += " AND action = $" + strconv.Itoa(idx)
		args = append(args, opts.Action)
		idx++
	}
	return clauses, args
}

// GetTradesByUserID returns a page of trades for a user, newest first.
// total is computed server-side as quantity * price.
func (uts *TradesStore) GetTradesByUserID(ctx context.Context, userID string, opts TradeQueryOpts) ([]Trade, error) {
	filter, filterArgs := buildTradeFilter(opts, 2)
	limitIdx := 2 + len(filterArgs)
	offsetIdx := limitIdx + 1

	query := `SELECT id, user_id, symbol, action, quantity, price, (quantity * price) AS total, executed_at, status, idempotency_key
		FROM trades
		WHERE user_id = $1` + filter + `
		ORDER BY executed_at DESC
		LIMIT $` + strconv.Itoa(limitIdx) + ` OFFSET $` + strconv.Itoa(offsetIdx)

	args := append([]interface{}{userID}, filterArgs...)
	args = append(args, opts.Limit, opts.Offset)

	rows, err := uts.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	trades := []Trade{}
	for rows.Next() {
		var t Trade
		var ikey sql.NullString
		if err := rows.Scan(&t.ID, &t.UserID, &t.Symbol, &t.Action, &t.Quantity, &t.Price, &t.Total, &t.ExecutedAt, &t.Status, &ikey); err != nil {
			return nil, err
		}
		if ikey.Valid {
			t.IdempotencyKey = ikey.String
		}
		trades = append(trades, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return trades, nil
}

// GetAllTradesByUserID fetches all trades for a user in chronological order
// (oldest first). Intended for internal use by the reconciliation service —
// not paginated and not exposed as an HTTP endpoint.
func (uts *TradesStore) GetAllTradesByUserID(ctx context.Context, userID string) ([]Trade, error) {
	query := `SELECT id, user_id, symbol, action, quantity, price, (quantity * price) AS total, executed_at, status, idempotency_key
		FROM trades
		WHERE user_id = $1
		ORDER BY executed_at ASC`

	rows, err := uts.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	trades := []Trade{}
	for rows.Next() {
		var t Trade
		var ikey sql.NullString
		if err := rows.Scan(&t.ID, &t.UserID, &t.Symbol, &t.Action, &t.Quantity, &t.Price, &t.Total, &t.ExecutedAt, &t.Status, &ikey); err != nil {
			return nil, err
		}
		if ikey.Valid {
			t.IdempotencyKey = ikey.String
		}
		trades = append(trades, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return trades, nil
}

// GetTradeByIdempotencyKey returns the trade for (userID, key), or (nil, nil)
// if no such key exists. Used to short-circuit duplicate buy/sell requests.
func (uts *TradesStore) GetTradeByIdempotencyKey(ctx context.Context, userID, key string) (*Trade, error) {
	query := `SELECT id, user_id, symbol, action, quantity, price, (quantity * price) AS total, executed_at, status, idempotency_key
		FROM trades
		WHERE user_id = $1 AND idempotency_key = $2`

	var trade Trade
	var ikey sql.NullString
	err := uts.db.QueryRowContext(ctx, query, userID, key).Scan(
		&trade.ID, &trade.UserID, &trade.Symbol, &trade.Action,
		&trade.Quantity, &trade.Price, &trade.Total, &trade.ExecutedAt,
		&trade.Status, &ikey,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if ikey.Valid {
		trade.IdempotencyKey = ikey.String
	}
	return &trade, nil
}

// CountTradesByUserID returns the total number of trades matching the filter,
// independent of limit/offset. Used by the API to render pagination state.
func (uts *TradesStore) CountTradesByUserID(ctx context.Context, userID string, opts TradeQueryOpts) (int, error) {
	filter, filterArgs := buildTradeFilter(opts, 2)
	query := `SELECT COUNT(*) FROM trades WHERE user_id = $1` + filter
	args := append([]interface{}{userID}, filterArgs...)

	var count int
	if err := uts.db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}
