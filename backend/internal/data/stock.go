package data

import (
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
)

type Stock struct {
	ID     string    `json:"id"`
	Symbol string    `json:"symbol"`
	Price  float64   `json:"price"`
	Date   time.Time `json:"date"`
}

type StockStore struct {
	db *sql.DB
}

func NewStockStore(db *sql.DB) *StockStore {
	return &StockStore{db: db}
}

func (ss *StockStore) Init() error {
	query := `
	CREATE TABLE IF NOT EXISTS stocks (
		id TEXT PRIMARY KEY,
		symbol TEXT NOT NULL,
		price REAL NOT NULL,
		date DATETIME NOT NULL
	)`

	_, err := ss.db.Exec(query)
	return err
}

func (ss *StockStore) CreateStock(stock *Stock) error {
	query := `
	INSERT INTO stocks (id, symbol, price, date) VALUES (?, ?, ?, ?)`

	_, err := ss.db.Exec(query, stock.ID, stock.Symbol, stock.Price, stock.Date)
	return err
}

func (ss *StockStore) GetStockByID(id string) (*Stock, error) {
	query := `SELECT id, symbol, price, date FROM stocks WHERE id = ?`

	var stock Stock
	err := ss.db.QueryRow(query, id).Scan(&stock.ID, &stock.Symbol, &stock.Price, &stock.Date)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("stock not found")
		}
		return nil, err
	}

	return &stock, nil
}

func (ss *StockStore) GetStockBySymbol(symbol string) (*Stock, error) {
	query := `SELECT id, symbol, price, date FROM stocks WHERE symbol = ?`

	var stock Stock
	err := ss.db.QueryRow(query, symbol).Scan(&stock.ID, &stock.Symbol, &stock.Price, &stock.Date)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("stock not found")
		}
		return nil, err
	}

	return &stock, nil
}

func (ss *StockStore) UpdateStockBySymbol(symbol string, newPrice float64, newDate time.Time) error {
	query := `UPDATE stocks SET price = ?, date = ? WHERE symbol = ?`

	result, err := ss.db.Exec(query, newPrice, newDate, symbol)
	if err != nil {
		return err
	}

	// Check if any rows were affected
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return errors.New("stock not found")
	}

	return nil
}

// UpdateOrCreateStockBySymbol updates existing stock or creates new one if it doesn't exist
func (ss *StockStore) UpdateOrCreateStockBySymbol(symbol string, price float64, date time.Time) error {
	// First try to update
	err := ss.UpdateStockBySymbol(symbol, price, date)
	if err != nil && err.Error() == "stock not found" {
		// If stock doesn't exist, create it
		stock := &Stock{
			ID:     generateStockID(), // You'll need to implement this
			Symbol: symbol,
			Price:  price,
			Date:   date,
		}
		return ss.CreateStock(stock)
	}
	return err
}

// Helper function to generate stock ID (you can implement this as needed)
func generateStockID() string {

	return uuid.New().String()
}
