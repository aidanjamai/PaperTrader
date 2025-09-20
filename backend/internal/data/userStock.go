package data

import (
	"database/sql"
	"errors"

	"github.com/google/uuid"
)

type UserStock struct {
	ID       string  `json:"id"`
	UserID   string  `json:"user_id"`
	Symbol   string  `json:"symbol"`
	Action   string  `json:"action"`
	Quantity int     `json:"quantity"`
	Price    float64 `json:"price"`
	Date     string  `json:"date"`
}

type UserStocksStore struct {
	db *sql.DB
}

func NewUserStocksStore(db *sql.DB) *UserStocksStore {
	return &UserStocksStore{db: db}
}

func (uss *UserStocksStore) Init() error {
	query := `CREATE TABLE IF NOT EXISTS user_stocks (
		id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL,
		symbol TEXT NOT NULL,
		action TEXT NOT NULL,
		quantity INTEGER NOT NULL,
		price REAL NOT NULL,
		date TEXT NOT NULL
	);`

	_, err := uss.db.Exec(query)
	return err
}

func (uss *UserStocksStore) CreateUserStockBuy(symbol string, quantity int, price float64, userID string, date string) error {
	userStock := &UserStock{
		ID:       generateUserStockID(),
		UserID:   userID,
		Symbol:   symbol,
		Action:   "BUY",
		Quantity: quantity,
		Price:    price,
		Date:     date,
	}

	return uss.CreateUserStock(userStock)
}

func (uss *UserStocksStore) CreateUserStockSell(symbol string, quantity int, price float64, userID string, date string) error {

	userStock := &UserStock{
		ID:       generateUserStockID(),
		UserID:   userID,
		Symbol:   symbol,
		Action:   "SELL",
		Quantity: quantity,
		Price:    price,
		Date:     date,
	}
	return uss.CreateUserStock(userStock)
}

func (uss *UserStocksStore) CreateUserStock(userStock *UserStock) error {
	query := `INSERT INTO user_stocks (id, user_id, symbol, action, quantity, price, date) VALUES (?, ?, ?, ?, ?, ?, ?)`
	_, err := uss.db.Exec(query, userStock.ID, userStock.UserID, userStock.Symbol, userStock.Action, userStock.Quantity, userStock.Price, userStock.Date)
	return err
}

func (uss *UserStocksStore) GetUserStockByID(id string) (*UserStock, error) {
	query := `SELECT id, user_id, symbol, action, quantity, price, date FROM user_stocks WHERE id = ?`

	var userStock UserStock
	err := uss.db.QueryRow(query, id).Scan(&userStock.ID, &userStock.UserID, &userStock.Symbol, &userStock.Action, &userStock.Quantity, &userStock.Price, &userStock.Date)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("user stock not found")
		}
		return nil, err
	}

	return &userStock, err
}

// func (uss *UserStocksStore) GetAllUserStocksByUserID(userID string) ([]UserStock, error) {
// 	query := `SELECT id, user_id, symbol, action, quantity, price, date FROM user_stocks WHERE user_id = ?`

// 	var userStocks []UserStock
// 	err := uss.db.QueryRow(query, userID).Scan(&userStocks.ID, &userStocks.UserID, &userStocks.Symbol, &userStocks.Action, &userStocks.Quantity, &userStocks.Price, &userStocks.Date)
// 	if err != nil {
// 		if err == sql.ErrNoRows {
// 			return nil, errors.New("user stocks not found")
// 		}
// 		return nil, err
// 	}

// 	return userStocks, nil
// }

func (uss *UserStocksStore) GetAllUserStocksByUserIDAndSymbol(userID string, symbol string) (*UserStock, error) {
	query := `SELECT id, user_id, symbol, action, quantity, price, date FROM user_stocks WHERE user_id = ? AND symbol = ?`

	var userStock UserStock
	err := uss.db.QueryRow(query, userID, symbol).Scan(&userStock.ID, &userStock.UserID, &userStock.Symbol, &userStock.Action, &userStock.Quantity, &userStock.Price, &userStock.Date)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("user stock not found")
		}
		return nil, err
	}
	return &userStock, nil
}

func (uss *UserStocksStore) DeleteUserStockByID(id string) error {
	query := `DELETE FROM user_stocks WHERE id = ?`
	_, err := uss.db.Exec(query, id)
	return err
}

func (uss *UserStocksStore) DeleteAllUserStocks() error {
	query := `DELETE FROM user_stocks`
	_, err := uss.db.Exec(query)
	return err
}

func generateUserStockID() string {

	return uuid.New().String()
}
