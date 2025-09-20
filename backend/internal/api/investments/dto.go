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

type UserStockResponse struct {
	ID       string  `json:"id"`
	Symbol   string  `json:"symbol"`
	Action   string  `json:"action"`
	Quantity int     `json:"quantity"`
	Price    float64 `json:"price"`
	Date     string  `json:"date"`
}
