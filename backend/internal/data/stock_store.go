package data

import "time"

type Stocks interface {
	Init() error
	CreateStock(stock *Stock) error
	GetStockByID(id string) (*Stock, error)
	GetStockBySymbol(symbol string) (*Stock, error)
	UpdateStockBySymbol(symbol string, newPrice float64, newDate time.Time) error
	UpdateOrCreateStockBySymbol(symbol string, price float64, date time.Time) error
}
