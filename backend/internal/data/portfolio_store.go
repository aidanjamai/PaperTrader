package data

import (
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
)

// UserStock represents a user's stock holding (moved from collections package)
type UserStock struct {
	ID                string    `json:"id"`
	UserID            string    `json:"user_id"`
	Symbol            string    `json:"symbol"`
	Quantity          int       `json:"quantity"`
	AvgPrice          float64   `json:"avg_price"`
	Total             float64   `json:"total"`
	CurrentStockPrice float64   `json:"current_stock_price"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

var ErrStockHoldingNotFound = errors.New("stock holding not found")

type PortfolioStore struct {
	db DBTX
}

func NewPortfolioStore(db DBTX) *PortfolioStore {
	return &PortfolioStore{db: db}
}

func (ps *PortfolioStore) Init() error {
	// Create table
	query := `
	CREATE TABLE IF NOT EXISTS portfolio (
		id VARCHAR(255) PRIMARY KEY,
		user_id VARCHAR(255) NOT NULL,
		symbol VARCHAR(10) NOT NULL,
		quantity INTEGER NOT NULL DEFAULT 0,
		avg_price NUMERIC(15,2) NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(user_id, symbol)
	);`

	_, err := ps.db.Exec(query)
	if err != nil {
		return err
	}

	// Create index separately (PostgreSQL CREATE INDEX IF NOT EXISTS syntax)
	indexQuery := `CREATE INDEX IF NOT EXISTS idx_portfolio_user_id ON portfolio(user_id);`
	_, err = ps.db.Exec(indexQuery)
	return err
}

// UpdatePortfolioWithBuy updates portfolio on buy order
// Uses INSERT ... ON CONFLICT for atomic upsert in PostgreSQL
func (ps *PortfolioStore) UpdatePortfolioWithBuy(userID, symbol string, quantity int, price float64) error {
	// Check if portfolio entry exists
	existing, err := ps.GetPortfolioBySymbol(userID, symbol)
	if err != nil && err != ErrStockHoldingNotFound {
		return err
	}

	var newQuantity int
	var newAvgPrice float64
	var portfolioID string

	if existing == nil {
		// New holding
		portfolioID = uuid.New().String()
		newQuantity = quantity
		newAvgPrice = price
	} else {
		// Existing holding - calculate weighted average
		portfolioID = existing.ID
		originalQuantity := existing.Quantity
		newQuantity = existing.Quantity + quantity
		newAvgPrice = (existing.AvgPrice*float64(originalQuantity) + price*float64(quantity)) / float64(newQuantity)
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

	_, err = ps.db.Exec(query, portfolioID, userID, symbol, newQuantity, newAvgPrice)
	return err
}

// UpdatePortfolioWithSell updates portfolio on sell order
func (ps *PortfolioStore) UpdatePortfolioWithSell(userID, symbol string, quantity int) error {
	existing, err := ps.GetPortfolioBySymbol(userID, symbol)
	if err != nil {
		if err == ErrStockHoldingNotFound {
			return errors.New("stock holding not found")
		}
		return err
	}

	if quantity > existing.Quantity {
		return errors.New("insufficient stock quantity to sell")
	}

	newQuantity := existing.Quantity - quantity

	if newQuantity == 0 {
		// Delete the entry if quantity reaches zero
		return ps.DeletePortfolio(userID, symbol)
	}

	// Update quantity, keep avg_price unchanged
	query := `UPDATE portfolio SET quantity = $1, updated_at = CURRENT_TIMESTAMP WHERE user_id = $2 AND symbol = $3`
	_, err = ps.db.Exec(query, newQuantity, userID, symbol)
	return err
}

// GetPortfolioByUserID gets all holdings for a user
func (ps *PortfolioStore) GetPortfolioByUserID(userID string) ([]UserStock, error) {
	query := `SELECT id, user_id, symbol, quantity, avg_price, created_at, updated_at 
	          FROM portfolio WHERE user_id = $1 AND quantity > 0 ORDER BY symbol`

	rows, err := ps.db.Query(query, userID)
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
		holding.Total = holding.AvgPrice * float64(holding.Quantity)
		holdings = append(holdings, holding)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return holdings, nil
}

// GetPortfolioBySymbol gets a specific holding
func (ps *PortfolioStore) GetPortfolioBySymbol(userID, symbol string) (*UserStock, error) {
	query := `SELECT id, user_id, symbol, quantity, avg_price, created_at, updated_at 
	          FROM portfolio WHERE user_id = $1 AND symbol = $2`

	var holding UserStock
	err := ps.db.QueryRow(query, userID, symbol).Scan(
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

	// Calculate derived fields
	holding.Total = holding.AvgPrice * float64(holding.Quantity)

	return &holding, nil
}

// DeletePortfolio removes a portfolio entry
func (ps *PortfolioStore) DeletePortfolio(userID, symbol string) error {
	query := `DELETE FROM portfolio WHERE user_id = $1 AND symbol = $2`
	result, err := ps.db.Exec(query, userID, symbol)
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
func (ps *PortfolioStore) DeleteAllPortfolio(userID string) error {
	query := `DELETE FROM portfolio WHERE user_id = $1`
	_, err := ps.db.Exec(query, userID)
	return err
}

