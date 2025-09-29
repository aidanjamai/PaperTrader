package collections

type UserStocks interface {
	Init() error
	CreateUserStock(userStock *UserStock) (*UserStock, error)
	UpdateUserStockWithBuy(userStock *UserStock) error
	UpdateUserStockWithSell(userStock *UserStock) error
	GetUserStocksByUserID(userID string) ([]UserStock, error)
	GetUserStockBySymbol(userID, symbol string) (*UserStock, error)
	DeleteUserStock(userID, symbol string) error
	DeleteAllUserStocks(userID string) error
}
