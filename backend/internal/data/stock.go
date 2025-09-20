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

func (ss *StockStore) GetStockBySymbolAndDate(symbol string, date string) (*Stock, error) {
	log.Printf("GetStockBySymbolAndDate: Symbol=%s, Date=%s", symbol, date)

	query := `SELECT id, symbol, price, date FROM stocks WHERE symbol = ? AND date = ?`
	//log.Printf("Executing query: %s with params: symbol=%s, date=%s", query, symbol, date)

	var stock Stock
	err := ss.db.QueryRow(query, symbol, date).Scan(&stock.ID, &stock.Symbol, &stock.Price, &stock.Date)

	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("No stock found for symbol=%s, date=%s", symbol, date)
			return nil, nil // Return nil stock, nil error for "not found"
		}
		log.Printf("Error querying stock: %v", err)
		return nil, err
	}

	log.Printf("Found stock: ID=%s, Symbol=%s, Price=%f, Date=%s",
		stock.ID, stock.Symbol, stock.Price, stock.Date)
	return &stock, nil
}

func (ss *StockStore) UpdateStockBySymbol(symbol string, newPrice float64, newDate string) error {
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
	log.Printf("Starting GetAllStocks query")

	query := `SELECT id, symbol, price, date FROM stocks`

	rows, err := ss.db.Query(query)
	if err != nil {
		log.Printf("Error executing query: %v", err)
		return nil, err
	}
	defer rows.Close()

	var stocks []Stock
	count := 0

	for rows.Next() {
		var stock Stock
		err := rows.Scan(&stock.ID, &stock.Symbol, &stock.Price, &stock.Date)
		if err != nil {
			log.Printf("Error scanning row %d: %v", count, err)
			return nil, err
		}
		//log.Printf("Scanned stock %d: ID=%s, Symbol=%s, Price=%f, Date=%s",
		//	count, stock.ID, stock.Symbol, stock.Price, stock.Date)
		stocks = append(stocks, stock)
		count++
	}

	if err = rows.Err(); err != nil {
		log.Printf("Error during row iteration: %v", err)
		return nil, err
	}

	log.Printf("GetAllStocks completed successfully. Found %d stocks", len(stocks))
	return stocks, nil
}

// Helper function to generate stock ID
func generateStockID() string {

	return uuid.New().String()
}

// Delete stock by id
func (ss *StockStore) DeleteStockById(id string) error {
	log.Printf("DeleteStockById: Starting deletion for ID=%s", id)

	if id == "" {
		log.Printf("DeleteStockById: Error - ID is empty")
		return errors.New("stock ID is required")
	}

	query := `DELETE FROM stocks WHERE id = ?`
	log.Printf("DeleteStockById: Executing query: %s with ID=%s", query, id)

	result, err := ss.db.Exec(query, id)
	if err != nil {
		log.Printf("DeleteStockById: Error executing query: %v", err)
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("DeleteStockById: Error getting rows affected: %v", err)
		return err
	}

	if rowsAffected == 0 {
		log.Printf("DeleteStockById: Warning - No stock found with ID=%s", id)
		return errors.New("stock not found")
	}

	log.Printf("DeleteStockById: Successfully deleted stock with ID=%s (rows affected: %d)", id, rowsAffected)
	return nil
}

// Delete stock by symbol
func (ss *StockStore) DeleteStockBySymbol(symbol string) error {
	log.Printf("DeleteStockBySymbol: Starting deletion for Symbol=%s", symbol)

	if symbol == "" {
		log.Printf("DeleteStockBySymbol: Error - Symbol is empty")
		return errors.New("stock symbol is required")
	}

	query := `DELETE FROM stocks WHERE symbol = ?`
	log.Printf("DeleteStockBySymbol: Executing query: %s with Symbol=%s", query, symbol)

	result, err := ss.db.Exec(query, symbol)
	if err != nil {
		log.Printf("DeleteStockBySymbol: Error executing query: %v", err)
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("DeleteStockBySymbol: Error getting rows affected: %v", err)
		return err
	}

	if rowsAffected == 0 {
		log.Printf("DeleteStockBySymbol: Warning - No stock found with Symbol=%s", symbol)
		return errors.New("stock not found")
	}

	log.Printf("DeleteStockBySymbol: Successfully deleted %d stock(s) with Symbol=%s", rowsAffected, symbol)
	return nil
}

// Delete all stocks
func (ss *StockStore) DeleteAllStocks() error {
	log.Printf("DeleteAllStocks: Starting deletion of all stocks")

	// First, get count of stocks before deletion
	countQuery := `SELECT COUNT(*) FROM stocks`
	var count int
	err := ss.db.QueryRow(countQuery).Scan(&count)
	if err != nil {
		log.Printf("DeleteAllStocks: Error getting stock count: %v", err)
		// Continue with deletion even if count fails
	} else {
		log.Printf("DeleteAllStocks: Found %d stocks to delete", count)
	}

	query := `DELETE FROM stocks`
	log.Printf("DeleteAllStocks: Executing query")

	result, err := ss.db.Exec(query)
	if err != nil {
		log.Printf("DeleteAllStocks: Error executing query: %v", err)
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("DeleteAllStocks: Error getting rows affected: %v", err)
		return err
	}

	log.Printf("DeleteAllStocks: Successfully deleted all stocks (rows affected: %d)", rowsAffected)
	return nil
}
