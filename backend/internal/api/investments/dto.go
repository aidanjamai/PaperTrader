package investments

type BuyStockRequest struct {
	UserID   string `json:"user_id"`
	Symbol   string `json:"symbol"`
	Quantity int    `json:"quantity"`
}

type SellStockRequest struct {
	UserID   string `json:"user_id"`
	Symbol   string `json:"symbol"`
	Quantity int    `json:"quantity"`
}

type TradeResponse struct {
	ID       string  `json:"id"`
	Symbol   string  `json:"symbol"`
	Action   string  `json:"action"`
	Quantity int     `json:"quantity"`
	Price    float64 `json:"price"`
	Date     string  `json:"date"`
}

type UserStock struct {
	ID       string  `json:"id"`
	UserID   string  `json:"user_id"`
	Symbol   string  `json:"symbol"`
	Quantity int     `json:"quantity"`
	AvgPrice float64 `json:"avg_price"`
	Total    float64 `json:"total"`
	Profit   float64 `json:"profit"`
}
