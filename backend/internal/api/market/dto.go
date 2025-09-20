package market

type PostStockRequest struct {
	Symbol string  `json:"symbol"`
	Price  float64 `json:"price"`
}

type StockResponse struct {
	Symbol string  `json:"symbol"`
	Date   string  `json:"date"`
	Price  float64 `json:"price"`
}

type StockIdRequest struct {
	ID string `json:"id"`
}

type StockSymbolRequest struct {
	Symbol string `json:"symbol"`
}
