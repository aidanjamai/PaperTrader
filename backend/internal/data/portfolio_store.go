package data

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// UserStock represents a user's stock holding (moved from collections package)
type UserStock struct {
	ID                string          `json:"id"`
	UserID            string          `json:"user_id"`
	Symbol            string          `json:"symbol"`
	Quantity          int             `json:"quantity"`
	AvgPrice          decimal.Decimal `json:"avg_price"`
	Total             decimal.Decimal `json:"total"`
	CurrentStockPrice decimal.Decimal `json:"current_stock_price"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
}

var ErrStockHoldingNotFound = errors.New("stock holding not found")

type PortfolioStore struct {
	db DBTX
}

func NewPortfolioStore(db DBTX) *PortfolioStore {
	return &PortfolioStore{db: db}
}

// UpdatePortfolioWithBuy updates portfolio on buy order. Uses
// SELECT ... FOR UPDATE to lock any existing row so two concurrent buys of the
// same (user, symbol) cannot both read stale quantity/avg_price and overwrite
// each other's weighted average. The user-balance lock in InvestmentService
// already serialises buys for the same user, but this lock keeps the store
// safe in isolation.
func (ps *PortfolioStore) UpdatePortfolioWithBuy(ctx context.Context, userID, symbol string, quantity int, price decimal.Decimal) error {
	existing, err := ps.GetPortfolioBySymbolForUpdate(ctx, userID, symbol)
	if err != nil && err != ErrStockHoldingNotFound {
		return err
	}

	var newQuantity int
	var newAvgPrice decimal.Decimal
	var portfolioID string

	if existing == nil {
		// New holding
		portfolioID = uuid.New().String()
		newQuantity = quantity
		newAvgPrice = price
	} else {
		// Existing holding - calculate weighted average using exact decimal arithmetic.
		portfolioID = existing.ID
		originalQuantity := existing.Quantity
		newQuantity = existing.Quantity + quantity
		existingTotal := existing.AvgPrice.Mul(decimal.NewFromInt(int64(originalQuantity)))
		addedTotal := price.Mul(decimal.NewFromInt(int64(quantity)))
		newAvgPrice = existingTotal.Add(addedTotal).Div(decimal.NewFromInt(int64(newQuantity)))
	}

	// Use PostgreSQL INSERT ... ON CONFLICT for atomic upsert
	query := `
	INSERT INTO portfolio (id, user_id, symbol, quantity, avg_price, updated_at)
	VALUES ($1, $2, $3, $4, $5, CURRENT_TIMESTAMP)
	ON CONFLICT (user_id, symbol)
	DO UPDATE SET
		quantity = EXCLUDED.quantity,
		avg_price = EXCLUDED.avg_price,
		updated_at = CURRENT_TIMESTAMP`

	_, err = ps.db.ExecContext(ctx, query, portfolioID, userID, symbol, newQuantity, newAvgPrice)
	return err
}

// UpdatePortfolioWithSell decrements an existing holding by `quantity`,
// deleting the row if the resulting quantity would be zero.
//
// The caller is responsible for fetching the current holding under FOR UPDATE
// (see GetPortfolioBySymbolForUpdate) and validating that
// quantity <= currentQuantity before calling this. Re-reading the row here
// would be either redundant (the caller's lock makes the value unchanged) or
// — if the lock is ever dropped — racy. Pass the locked currentQuantity in.
func (ps *PortfolioStore) UpdatePortfolioWithSell(ctx context.Context, userID, symbol string, currentQuantity, quantity int) error {
	if quantity > currentQuantity {
		return errors.New("insufficient stock quantity to sell")
	}

	newQuantity := currentQuantity - quantity
	if newQuantity == 0 {
		return ps.DeletePortfolio(ctx, userID, symbol)
	}

	query := `UPDATE portfolio SET quantity = $1, updated_at = CURRENT_TIMESTAMP WHERE user_id = $2 AND symbol = $3`
	_, err := ps.db.ExecContext(ctx, query, newQuantity, userID, symbol)
	return err
}

// GetPortfolioByUserID gets all holdings for a user
func (ps *PortfolioStore) GetPortfolioByUserID(ctx context.Context, userID string) ([]UserStock, error) {
	query := `SELECT id, user_id, symbol, quantity, avg_price, created_at, updated_at
	          FROM portfolio WHERE user_id = $1 AND quantity > 0 ORDER BY symbol`

	rows, err := ps.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var holdings []UserStock
	for rows.Next() {
		var holding UserStock
		err := rows.Scan(
			&holding.ID,
			&holding.UserID,
			&holding.Symbol,
			&holding.Quantity,
			&holding.AvgPrice,
			&holding.CreatedAt,
			&holding.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		// Calculate derived fields
		holding.Total = holding.AvgPrice.Mul(decimal.NewFromInt(int64(holding.Quantity)))
		holdings = append(holdings, holding)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return holdings, nil
}

// GetPortfolioBySymbol gets a specific holding
func (ps *PortfolioStore) GetPortfolioBySymbol(ctx context.Context, userID, symbol string) (*UserStock, error) {
	return ps.scanHolding(ctx, `SELECT id, user_id, symbol, quantity, avg_price, created_at, updated_at
	          FROM portfolio WHERE user_id = $1 AND symbol = $2`, userID, symbol)
}

// GetPortfolioBySymbolForUpdate is GetPortfolioBySymbol with FOR UPDATE so the
// caller's transaction holds the row lock until commit. Required inside
// SellStock — without it, two concurrent sells of the same holding can both
// pass the quantity check and oversell.
func (ps *PortfolioStore) GetPortfolioBySymbolForUpdate(ctx context.Context, userID, symbol string) (*UserStock, error) {
	return ps.scanHolding(ctx, `SELECT id, user_id, symbol, quantity, avg_price, created_at, updated_at
	          FROM portfolio WHERE user_id = $1 AND symbol = $2 FOR UPDATE`, userID, symbol)
}

func (ps *PortfolioStore) scanHolding(ctx context.Context, query, userID, symbol string) (*UserStock, error) {
	var holding UserStock
	err := ps.db.QueryRowContext(ctx, query, userID, symbol).Scan(
		&holding.ID,
		&holding.UserID,
		&holding.Symbol,
		&holding.Quantity,
		&holding.AvgPrice,
		&holding.CreatedAt,
		&holding.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrStockHoldingNotFound
		}
		return nil, err
	}
	holding.Total = holding.AvgPrice.Mul(decimal.NewFromInt(int64(holding.Quantity)))
	return &holding, nil
}

// DeletePortfolio removes a portfolio entry
func (ps *PortfolioStore) DeletePortfolio(ctx context.Context, userID, symbol string) error {
	query := `DELETE FROM portfolio WHERE user_id = $1 AND symbol = $2`
	result, err := ps.db.ExecContext(ctx, query, userID, symbol)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrStockHoldingNotFound
	}

	return nil
}

// DeleteAllPortfolio removes all holdings for a user
func (ps *PortfolioStore) DeleteAllPortfolio(ctx context.Context, userID string) error {
	query := `DELETE FROM portfolio WHERE user_id = $1`
	_, err := ps.db.ExecContext(ctx, query, userID)
	return err
}
