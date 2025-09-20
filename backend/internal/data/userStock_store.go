package data

type UserStocks interface {
	Init() error
	CreateUserStockBuy(symbol string, quantity int, price float64, userID string, date string) error
	CreateUserStockSell(symbol string, quantity int, price float64, userID string, date string) error
	CreateUserStock(userStock *UserStock) error
	GetUserStockByID(id string) (*UserStock, error)
	GetAllUserStocksByUserID(userID string) ([]UserStock, error)
	GetAllUserStocksByUserIDAndSymbol(userID string, symbol string) (*UserStock, error)
	DeleteUserStockByID(id string) error
	DeleteAllUserStocks() error
}
