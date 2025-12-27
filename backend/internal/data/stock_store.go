package data

import (
	"database/sql"
	"errors"
	"log"

	"github.com/google/uuid"
)

type Stock struct {
	ID     string  `json:"id"`
	Symbol string  `json:"symbol"`
	Price  float64 `json:"price"`
	Date   string  `json:"date"`
}

type StockStore struct {
	db DBTX
}

func NewStockStore(db DBTX) *StockStore {
	return &StockStore{db: db}
}

func (ss *StockStore) Init() error {
	query := `
	CREATE TABLE IF NOT EXISTS stocks (
		id VARCHAR(255) PRIMARY KEY,
		symbol VARCHAR(10) NOT NULL,
		price NUMERIC(15,2) NOT NULL,
		date TIMESTAMP NOT NULL
	)`

	_, err := ss.db.Exec(query)
	return err
}

func (ss *StockStore) CreateStock(stock *Stock) error {
	query := `
	INSERT INTO stocks (id, symbol, price, date) VALUES ($1, $2, $3, $4)`

	_, err := ss.db.Exec(query, stock.ID, stock.Symbol, stock.Price, stock.Date)
	return err
}

func (ss *StockStore) GetStockByID(id string) (*Stock, error) {
	query := `SELECT id, symbol, price, date FROM stocks WHERE id = $1`

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
	query := `SELECT id, symbol, price, date FROM stocks WHERE symbol = $1`

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

func (ss *StockStore) GetStockBySymbolAndDate(symbol string, date string) (*Stock, error) {
	query := `SELECT id, symbol, price, date FROM stocks WHERE symbol = $1 AND date = $2`

	var stock Stock
	err := ss.db.QueryRow(query, symbol, date).Scan(&stock.ID, &stock.Symbol, &stock.Price, &stock.Date)

	if err != nil {
		if err == sql.ErrNoRows {
			// This is a common cache miss scenario, no need to log as error
			return nil, nil
		}
		log.Printf("[StockStore] Error querying stock by symbol/date: %v", err)
		return nil, err
	}

	return &stock, nil
}

func (ss *StockStore) UpdateStockBySymbol(symbol string, newPrice float64, newDate string) error {
	query := `UPDATE stocks SET price = $1, date = $2 WHERE symbol = $3`

	result, err := ss.db.Exec(query, newPrice, newDate, symbol)
	if err != nil {
		return err
	}

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
func (ss *StockStore) UpdateOrCreateStockBySymbol(symbol string, price float64, date string) error {
	// First try to update
	err := ss.UpdateStockBySymbol(symbol, price, date)
	if err != nil && err.Error() == "stock not found" {
		// If stock doesn't exist, create it
		stock := &Stock{
			ID:     generateStockID(),
			Symbol: symbol,
			Price:  price,
			Date:   date,
		}
		return ss.CreateStock(stock)
	}
	return err
}

func (ss *StockStore) GetAllStocks() ([]Stock, error) {
	query := `SELECT id, symbol, price, date FROM stocks`

	rows, err := ss.db.Query(query)
	if err != nil {
		log.Printf("[StockStore] Error executing GetAllStocks query: %v", err)
		return nil, err
	}
	defer rows.Close()

	var stocks []Stock

	for rows.Next() {
		var stock Stock
		err := rows.Scan(&stock.ID, &stock.Symbol, &stock.Price, &stock.Date)
		if err != nil {
			log.Printf("[StockStore] Error scanning stock row: %v", err)
			return nil, err
		}
		stocks = append(stocks, stock)
	}

	if err = rows.Err(); err != nil {
		log.Printf("[StockStore] Error iterating stock rows: %v", err)
		return nil, err
	}

	return stocks, nil
}

// Helper function to generate stock ID
func generateStockID() string {
	return uuid.New().String()
}

// Delete stock by id
func (ss *StockStore) DeleteStockById(id string) error {
	if id == "" {
		return errors.New("stock ID is required")
	}

	query := `DELETE FROM stocks WHERE id = $1`
	result, err := ss.db.Exec(query, id)
	if err != nil {
		log.Printf("[StockStore] Error deleting stock by ID: %v", err)
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return errors.New("stock not found")
	}

	return nil
}

// Delete stock by symbol
func (ss *StockStore) DeleteStockBySymbol(symbol string) error {
	if symbol == "" {
		return errors.New("stock symbol is required")
	}

	query := `DELETE FROM stocks WHERE symbol = $1`
	result, err := ss.db.Exec(query, symbol)
	if err != nil {
		log.Printf("[StockStore] Error deleting stock by symbol: %v", err)
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return errors.New("stock not found")
	}

	return nil
}

// Delete all stocks
func (ss *StockStore) DeleteAllStocks() error {
	query := `DELETE FROM stocks`
	result, err := ss.db.Exec(query)
	if err != nil {
		log.Printf("[StockStore] Error deleting all stocks: %v", err)
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	log.Printf("[StockStore] Deleted all stocks (%d rows affected)", rowsAffected)
	return nil
}
