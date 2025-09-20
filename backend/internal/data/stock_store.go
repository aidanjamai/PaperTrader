package data

type Stocks interface {
	Init() error
	CreateStock(stock *Stock) error
	GetStockByID(id string) (*Stock, error)
	GetStockBySymbol(symbol string) (*Stock, error)
	UpdateStockBySymbol(symbol string, newPrice float64, newDate string) error
	UpdateOrCreateStockBySymbol(symbol string, price float64, date string) error
	GetStockBySymbolAndDate(symbol string, date string) (*Stock, error)
	GetAllStocks() ([]Stock, error)
	DeleteStockById(id string) error
	DeleteStockBySymbol(symbol string) error
	DeleteAllStocks() error
}
