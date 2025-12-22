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

type StockHistoricalDataDailyRequest struct {
	Symbol    string `json:"symbol"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
}

type StockHistoricalDataDailyResponse struct {
	Symbol           string  `json:"symbol"`
	Date             string  `json:"date"`
	PreviousPrice    float64 `json:"previous_price"`
	Price            float64 `json:"price"`
	Volume           int     `json:"volume"`
	Change           float64 `json:"change"`
	ChangePercentage float64 `json:"change_percentage"`
}

// MarketResponse is a generic success response
type MarketResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ErrorResponse is a generic error response
type ErrorResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
}
