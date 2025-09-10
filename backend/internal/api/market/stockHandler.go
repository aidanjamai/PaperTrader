package market

import (
	"fmt"
	"io"
	"net/http"
	"os"

	"papertrader/internal/data"
)

type stockHandler struct {
	stocks data.Stocks
}

func NewStockHandler(stocks data.Stocks) *stockHandler {
	return &stockHandler{stocks: stocks}
}

// Handler methods
func (h *stockHandler) GetStock(w http.ResponseWriter, r *http.Request) {
	var stockReq StockRequest
	//TODO: check db if stock exists in db for todays date
	//if it does, return the stock and dont make api request

	apiKey := os.Getenv("ALPHAVANTAGE_API_KEY")

	// Build request
	baseURL := "https://www.alphavantage.co/query"
	req, err := http.NewRequest("GET", baseURL, nil)
	if err != nil {
		panic(err)
	}

	// Add query params
	q := req.URL.Query()
	q.Add("function", stockReq.Function)
	q.Add("symbol", stockReq.Symbol)
	q.Add("interval", stockReq.Interval)
	q.Add("apikey", apiKey)
	req.URL.RawQuery = q.Encode()

	// (Optional) headers – usually not needed
	req.Header.Set("Accept", "application/json")

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	// Read and decode
	body, _ := io.ReadAll(resp.Body)

	fmt.Println(string(body))

}
