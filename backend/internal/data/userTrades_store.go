package data

type UserTrades interface {
	Init() error
	CreateUserTradeBuy(symbol string, quantity int, price float64, userID string, date string) error
	CreateUserTradeSell(symbol string, quantity int, price float64, userID string, date string) error
	CreateUserTrade(userTrade *UserTrade) error
	GetUserTradeByID(id string) (*UserTrade, error)
	GetAllUserTradesByUserID(userID string) ([]UserTrade, error)
	GetAllUserTradesByUserIDAndSymbol(userID string, symbol string) (*UserTrade, error)
	DeleteUserTradeByID(id string) error
	DeleteAllUserTrades() error
}
