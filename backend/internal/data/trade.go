package data

import (
	"database/sql"
	"errors"

	"github.com/google/uuid"
)

type Trade struct {
	ID       string  `json:"id"`
	UserID   string  `json:"user_id"`
	Symbol   string  `json:"symbol"`
	Action   string  `json:"action"`
	Quantity int     `json:"quantity"`
	Price    float64 `json:"price"`
	Date     string  `json:"date"`
}

type TradesStore struct {
	db *sql.DB
}

func NewTradesStore(db *sql.DB) *TradesStore {
	return &TradesStore{db: db}
}

func (uss *TradesStore) Init() error {
	query := `CREATE TABLE IF NOT EXISTS trades (
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

func (uss *TradesStore) CreateTradeBuy(symbol string, quantity int, price float64, userID string, date string) error {
	trade := &Trade{
		ID:       generateTradeID(),
		UserID:   userID,
		Symbol:   symbol,
		Action:   "BUY",
		Quantity: quantity,
		Price:    price,
		Date:     date,
	}

	return uss.CreateTrade(trade)
}

func (uss *TradesStore) CreateTradeSell(symbol string, quantity int, price float64, userID string, date string) error {

	trade := &Trade{
		ID:       generateTradeID(),
		UserID:   userID,
		Symbol:   symbol,
		Action:   "SELL",
		Quantity: quantity,
		Price:    price,
		Date:     date,
	}
	return uss.CreateTrade(trade)
}

func (uts *TradesStore) CreateTrade(trade *Trade) error {
	query := `INSERT INTO trades (id, user_id, symbol, action, quantity, price, date) VALUES (?, ?, ?, ?, ?, ?, ?)`
	_, err := uts.db.Exec(query, trade.ID, trade.UserID, trade.Symbol, trade.Action, trade.Quantity, trade.Price, trade.Date)
	return err
}

func (uts *TradesStore) GetTradeByID(id string) (*Trade, error) {
	query := `SELECT id, user_id, symbol, action, quantity, price, date FROM trades WHERE id = ?`

	var trade Trade
	err := uts.db.QueryRow(query, id).Scan(&trade.ID, &trade.UserID, &trade.Symbol, &trade.Action, &trade.Quantity, &trade.Price, &trade.Date)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("trade not found")
		}
		return nil, err
	}

	return &trade, err
}

// func (uts *UserTradesStore) GetAllUserTradesByUserID(userID string) ([]UserTrade, error) {
// 	query := `SELECT id, user_id, symbol, action, quantity, price, date FROM trades WHERE user_id = ?`

// 	var userTrades []Trade
// 	err := uts.db.QueryRow(query, userID).Scan(&userTrades.ID, &userTrades.UserID, &userTrades.Symbol, &userTrades.Action, &userTrades.Quantity, &userTrades.Price, &userTrades.Date)
// 	if err != nil {
// 		if err == sql.ErrNoRows {
// 			return nil, errors.New("user trades not found")
// 		}
// 		return nil, err
// 	}

// 	return userTrades, nil
// }

func (uts *TradesStore) GetAllTradesByUserIDAndSymbol(userID string, symbol string) (*Trade, error) {
	query := `SELECT id, user_id, symbol, action, quantity, price, date FROM trades WHERE user_id = ? AND symbol = ?`

	var trade Trade
	err := uts.db.QueryRow(query, userID, symbol).Scan(&trade.ID, &trade.UserID, &trade.Symbol, &trade.Action, &trade.Quantity, &trade.Price, &trade.Date)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("trade not found")
		}
		return nil, err
	}
	return &trade, nil
}

func (uts *TradesStore) DeleteTradeByID(id string) error {
	query := `DELETE FROM trades WHERE id = ?`
	_, err := uts.db.Exec(query, id)
	return err
}

func (uts *TradesStore) DeleteAllTrades() error {
	query := `DELETE FROM trades`
	_, err := uts.db.Exec(query)
	return err
}

func generateTradeID() string {

	return uuid.New().String()
}
