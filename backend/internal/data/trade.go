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
	Status   string  `json:"status"` // PENDING, COMPLETED, FAILED
}

type TradesStore struct {
	db DBTX
}

func NewTradesStore(db DBTX) *TradesStore {
	return &TradesStore{db: db}
}

func (uss *TradesStore) Init() error {
	query := `CREATE TABLE IF NOT EXISTS trades (
		id VARCHAR(255) PRIMARY KEY,
		user_id VARCHAR(255) NOT NULL,
		symbol VARCHAR(10) NOT NULL,
		action VARCHAR(10) NOT NULL,
		quantity INTEGER NOT NULL,
		price NUMERIC(15,2) NOT NULL,
		date VARCHAR(20) NOT NULL,
		status VARCHAR(20) NOT NULL DEFAULT 'COMPLETED'
	);`

	_, err := uss.db.Exec(query)
	return err
}

func (uss *TradesStore) CreateTradeBuy(symbol string, quantity int, price float64, userID string, date string) error {
	trade := &Trade{
		ID:       GenerateTradeID(),
		UserID:   userID,
		Symbol:   symbol,
		Action:   "BUY",
		Quantity: quantity,
		Price:    price,
		Date:     date,
		Status:   "COMPLETED",
	}

	return uss.CreateTrade(trade)
}

func (uss *TradesStore) CreateTradeSell(symbol string, quantity int, price float64, userID string, date string) error {
	trade := &Trade{
		ID:       GenerateTradeID(),
		UserID:   userID,
		Symbol:   symbol,
		Action:   "SELL",
		Quantity: quantity,
		Price:    price,
		Date:     date,
		Status:   "COMPLETED",
	}
	return uss.CreateTrade(trade)
}

func (uts *TradesStore) CreateTrade(trade *Trade) error {
	if trade.Status == "" {
		trade.Status = "COMPLETED"
	}
	query := `INSERT INTO trades (id, user_id, symbol, action, quantity, price, date, status) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
	_, err := uts.db.Exec(query, trade.ID, trade.UserID, trade.Symbol, trade.Action, trade.Quantity, trade.Price, trade.Date, trade.Status)
	return err
}

func (uts *TradesStore) UpdateTradeStatus(id string, status string) error {
	query := `UPDATE trades SET status = $1 WHERE id = $2`
	_, err := uts.db.Exec(query, status, id)
	return err
}

func (uts *TradesStore) GetTradeByID(id string) (*Trade, error) {
	query := `SELECT id, user_id, symbol, action, quantity, price, date, status FROM trades WHERE id = $1`

	var trade Trade
	err := uts.db.QueryRow(query, id).Scan(&trade.ID, &trade.UserID, &trade.Symbol, &trade.Action, &trade.Quantity, &trade.Price, &trade.Date, &trade.Status)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("trade not found")
		}
		return nil, err
	}

	return &trade, err
}

func (uts *TradesStore) GetAllTradesByUserIDAndSymbol(userID string, symbol string) (*Trade, error) {
	query := `SELECT id, user_id, symbol, action, quantity, price, date, status FROM trades WHERE user_id = $1 AND symbol = $2`

	var trade Trade
	err := uts.db.QueryRow(query, userID, symbol).Scan(&trade.ID, &trade.UserID, &trade.Symbol, &trade.Action, &trade.Quantity, &trade.Price, &trade.Date, &trade.Status)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("trade not found")
		}
		return nil, err
	}
	return &trade, nil
}

func (uts *TradesStore) DeleteTradeByID(id string) error {
	query := `DELETE FROM trades WHERE id = $1`
	_, err := uts.db.Exec(query, id)
	return err
}

func (uts *TradesStore) DeleteAllTrades() error {
	query := `DELETE FROM trades`
	_, err := uts.db.Exec(query)
	return err
}

func GenerateTradeID() string {
	return uuid.New().String()
}
