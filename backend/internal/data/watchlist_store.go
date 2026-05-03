package data

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

type WatchlistEntry struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Symbol    string    `json:"symbol"`
	CreatedAt time.Time `json:"created_at"`
}

var (
	ErrWatchlistEntryNotFound = errors.New("watchlist entry not found")
	ErrWatchlistEntryExists   = errors.New("watchlist entry already exists")
)

type WatchlistStore struct {
	db DBTX
}

func NewWatchlistStore(db DBTX) *WatchlistStore {
	return &WatchlistStore{db: db}
}

// Add inserts a new watchlist entry. Returns ErrWatchlistEntryExists if (user_id, symbol)
// already exists.
func (ws *WatchlistStore) Add(ctx context.Context, userID, symbol string) (*WatchlistEntry, error) {
	id := uuid.New().String()
	query := `
	INSERT INTO watchlist (id, user_id, symbol)
	VALUES ($1, $2, $3)
	RETURNING id, user_id, symbol, created_at`

	var entry WatchlistEntry
	err := ws.db.QueryRowContext(ctx, query, id, userID, symbol).Scan(
		&entry.ID,
		&entry.UserID,
		&entry.Symbol,
		&entry.CreatedAt,
	)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return nil, ErrWatchlistEntryExists
		}
		return nil, err
	}
	return &entry, nil
}

// Remove deletes the watchlist entry matching (user_id, symbol). Returns
// ErrWatchlistEntryNotFound if no row was deleted.
func (ws *WatchlistStore) Remove(ctx context.Context, userID, symbol string) error {
	query := `DELETE FROM watchlist WHERE user_id = $1 AND symbol = $2`
	result, err := ws.db.ExecContext(ctx, query, userID, symbol)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrWatchlistEntryNotFound
	}
	return nil
}

// ListByUser returns all watchlist entries for a user, ordered by symbol.
func (ws *WatchlistStore) ListByUser(ctx context.Context, userID string) ([]WatchlistEntry, error) {
	query := `SELECT id, user_id, symbol, created_at
	          FROM watchlist WHERE user_id = $1 ORDER BY symbol`

	rows, err := ws.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []WatchlistEntry
	for rows.Next() {
		var e WatchlistEntry
		if err := rows.Scan(&e.ID, &e.UserID, &e.Symbol, &e.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}
