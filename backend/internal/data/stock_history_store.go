package data

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

// StockHistoryPoint is one persisted EOD close.
type StockHistoryPoint struct {
	Symbol    string
	TradeDate time.Time
	Close     decimal.Decimal
	Volume    int64
}

type StockHistoryStore struct {
	db DBTX
}

func NewStockHistoryStore(db DBTX) *StockHistoryStore {
	return &StockHistoryStore{db: db}
}

// GetRange returns all stored closes for symbol with trade_date in [from, to],
// ordered by trade_date ASC. Empty slice (not nil) when there are no rows.
func (s *StockHistoryStore) GetRange(ctx context.Context, symbol string, from, to time.Time) ([]StockHistoryPoint, error) {
	const query = `
		SELECT symbol, trade_date, close, volume
		FROM stock_history
		WHERE symbol = $1 AND trade_date >= $2 AND trade_date <= $3
		ORDER BY trade_date ASC`

	rows, err := s.db.QueryContext(ctx, query, symbol, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]StockHistoryPoint, 0, 256)
	for rows.Next() {
		var p StockHistoryPoint
		if err := rows.Scan(&p.Symbol, &p.TradeDate, &p.Close, &p.Volume); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// upsertBatchSize bounds how many rows are sent in a single multi-VALUES
// statement. Postgres caps prepared-statement parameters at 65535, and we use
// 4 params per row, so the absolute ceiling is 16383. We pick a far smaller
// value so a single batch stays well under the planner's row-list cost cliff.
const upsertBatchSize = 1000

// UpsertMany inserts or updates points in batches of at most upsertBatchSize.
// On conflict on (symbol, trade_date), close/volume/fetched_at are refreshed.
// No-op on empty input. Returns the first error encountered; partial progress
// is possible if the caller does not run inside a transaction.
func (s *StockHistoryStore) UpsertMany(ctx context.Context, points []StockHistoryPoint) error {
	if len(points) == 0 {
		return nil
	}

	for i := 0; i < len(points); i += upsertBatchSize {
		end := i + upsertBatchSize
		if end > len(points) {
			end = len(points)
		}
		if err := s.upsertChunk(ctx, points[i:end]); err != nil {
			return err
		}
	}
	return nil
}

func (s *StockHistoryStore) upsertChunk(ctx context.Context, points []StockHistoryPoint) error {
	var b strings.Builder
	b.WriteString("INSERT INTO stock_history (symbol, trade_date, close, volume) VALUES ")

	args := make([]any, 0, len(points)*4)
	for i, p := range points {
		if i > 0 {
			b.WriteString(",")
		}
		base := i * 4
		b.WriteString("($")
		b.WriteString(strconv.Itoa(base + 1))
		b.WriteString(",$")
		b.WriteString(strconv.Itoa(base + 2))
		b.WriteString(",$")
		b.WriteString(strconv.Itoa(base + 3))
		b.WriteString(",$")
		b.WriteString(strconv.Itoa(base + 4))
		b.WriteString(")")
		args = append(args, p.Symbol, p.TradeDate, p.Close, p.Volume)
	}
	b.WriteString(`
		ON CONFLICT (symbol, trade_date) DO UPDATE
		SET close = EXCLUDED.close,
		    volume = EXCLUDED.volume,
		    fetched_at = CURRENT_TIMESTAMP`)

	_, err := s.db.ExecContext(ctx, b.String(), args...)
	return err
}

