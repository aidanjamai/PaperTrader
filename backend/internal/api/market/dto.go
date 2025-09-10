package market

type StockRequest struct {
	Function   string `json:"function"`
	Symbol     string `json:"symbol"`
	Interval   string `json:"interval"`
	OutputSize string `json:"outputsize"`
	DataType   string `json:"datatype"`
}

type StockResponse struct {
	Symbol string  `json:"symbol"`
	Date   string  `json:"date"`
	Price  float32 `json:"price"`
}
