package data

type Trades interface {
	Init() error
	CreateTradeBuy(symbol string, quantity int, price float64, userID string, date string) error
	CreateTradeSell(symbol string, quantity int, price float64, userID string, date string) error
	CreateTrade(trade *Trade) error
	GetTradeByID(id string) (*Trade, error)
	GetAllTradesByUserID(userID string) ([]Trade, error)
	GetAllTradesByUserIDAndSymbol(userID string, symbol string) (*Trade, error)
	DeleteTradeByID(id string) error
	//TODO: delete last trade by userID and symbol
	DeleteAllTrades() error
}
