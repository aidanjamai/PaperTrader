package data

import (
	"database/sql"
	"errors"

	"github.com/google/uuid"
)

type UserTrade struct {
	ID       string  `json:"id"`
	UserID   string  `json:"user_id"`
	Symbol   string  `json:"symbol"`
	Action   string  `json:"action"`
	Quantity int     `json:"quantity"`
	Price    float64 `json:"price"`
	Date     string  `json:"date"`
}

type UserTradesStore struct {
	db *sql.DB
}

func NewUserTradesStore(db *sql.DB) *UserTradesStore {
	return &UserTradesStore{db: db}
}

func (uss *UserTradesStore) Init() error {
	query := `CREATE TABLE IF NOT EXISTS user_trades (
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

func (uss *UserTradesStore) CreateUserTradeBuy(symbol string, quantity int, price float64, userID string, date string) error {
	userTrade := &UserTrade{
		ID:       generateUserTradeID(),
		UserID:   userID,
		Symbol:   symbol,
		Action:   "BUY",
		Quantity: quantity,
		Price:    price,
		Date:     date,
	}

	return uss.CreateUserTrade(userTrade)
}

func (uss *UserTradesStore) CreateUserTradeSell(symbol string, quantity int, price float64, userID string, date string) error {

	userTrade := &UserTrade{
		ID:       generateUserTradeID(),
		UserID:   userID,
		Symbol:   symbol,
		Action:   "SELL",
		Quantity: quantity,
		Price:    price,
		Date:     date,
	}
	return uss.CreateUserTrade(userTrade)
}

func (uts *UserTradesStore) CreateUserTrade(userTrade *UserTrade) error {
	query := `INSERT INTO user_trades (id, user_id, symbol, action, quantity, price, date) VALUES (?, ?, ?, ?, ?, ?, ?)`
	_, err := uts.db.Exec(query, userTrade.ID, userTrade.UserID, userTrade.Symbol, userTrade.Action, userTrade.Quantity, userTrade.Price, userTrade.Date)
	return err
}

func (uts *UserTradesStore) GetUserTradeByID(id string) (*UserTrade, error) {
	query := `SELECT id, user_id, symbol, action, quantity, price, date FROM user_trades WHERE id = ?`

	var userTrade UserTrade
	err := uts.db.QueryRow(query, id).Scan(&userTrade.ID, &userTrade.UserID, &userTrade.Symbol, &userTrade.Action, &userTrade.Quantity, &userTrade.Price, &userTrade.Date)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("user trade not found")
		}
		return nil, err
	}

	return &userTrade, err
}

// func (uts *UserTradesStore) GetAllUserTradesByUserID(userID string) ([]UserTrade, error) {
// 	query := `SELECT id, user_id, symbol, action, quantity, price, date FROM user_trades WHERE user_id = ?`

// 	var userTrades []UserTrade
// 	err := uts.db.QueryRow(query, userID).Scan(&userTrades.ID, &userTrades.UserID, &userTrades.Symbol, &userTrades.Action, &userTrades.Quantity, &userTrades.Price, &userTrades.Date)
// 	if err != nil {
// 		if err == sql.ErrNoRows {
// 			return nil, errors.New("user trades not found")
// 		}
// 		return nil, err
// 	}

// 	return userTrades, nil
// }

func (uts *UserTradesStore) GetAllUserTradesByUserIDAndSymbol(userID string, symbol string) (*UserTrade, error) {
	query := `SELECT id, user_id, symbol, action, quantity, price, date FROM user_trades WHERE user_id = ? AND symbol = ?`

	var userTrade UserTrade
	err := uts.db.QueryRow(query, userID, symbol).Scan(&userTrade.ID, &userTrade.UserID, &userTrade.Symbol, &userTrade.Action, &userTrade.Quantity, &userTrade.Price, &userTrade.Date)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("user trade not found")
		}
		return nil, err
	}
	return &userTrade, nil
}

func (uts *UserTradesStore) DeleteUserTradeByID(id string) error {
	query := `DELETE FROM user_trades WHERE id = ?`
	_, err := uts.db.Exec(query, id)
	return err
}

func (uts *UserTradesStore) DeleteAllUserTrades() error {
	query := `DELETE FROM user_trades`
	_, err := uts.db.Exec(query)
	return err
}

func generateUserTradeID() string {

	return uuid.New().String()
}
