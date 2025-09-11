package market

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"papertrader/internal/data"
)

// Create a generic response structure for market handlers
type MarketResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Generic error response
type ErrorResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
}

type stockHandler struct {
	stocks data.Stocks
}

func NewStockHandler(stocks data.Stocks) *stockHandler {
	return &stockHandler{stocks: stocks}
}

func (h *stockHandler) writeJSONResponse(w http.ResponseWriter, statusCode int, response interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}

func (h *stockHandler) writeSuccessResponse(w http.ResponseWriter, statusCode int, message string, data interface{}) {
	response := MarketResponse{
		Success: true,
		Message: message,
		Data:    data,
	}
	h.writeJSONResponse(w, statusCode, response)
}

func (h *stockHandler) writeErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	response := ErrorResponse{
		Success: false,
		Message: message,
	}
	h.writeJSONResponse(w, statusCode, response)
}

func (h *stockHandler) validateStockRequest(req *StockRequest) error {
	if req.Symbol == "" {
		return fmt.Errorf("symbol is required")
	}
	if req.Function == "" {
		return fmt.Errorf("function is required")
	}
	if req.Interval == "" {
		return fmt.Errorf("interval is required")
	}
	return nil
}

// Handler methods
func (h *stockHandler) GetStock(w http.ResponseWriter, r *http.Request) {
	var stockReq StockRequest

	// Validate request method
	// if r.Method != http.MethodPost {
	// 	h.writeErrorResponse(w, http.StatusMethodNotAllowed, "Only POST method allowed")
	// 	return
	// }

	// Decode and validate request
	if err := json.NewDecoder(r.Body).Decode(&stockReq); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid JSON format")
		return
	}

	// Validate required fields
	if err := h.validateStockRequest(&stockReq); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	//check db if stock exists in db for todays date
	//if it does, return the stock and dont make api request
	stock, err := h.stocks.GetStockBySymbolAndDate(stockReq.Symbol, time.Now())
	if err != nil {
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to get stock data")
		return
	}
	if stock != nil {
		h.writeSuccessResponse(w, http.StatusOK, "Stock data retrieved successfully", stock)
		return
	}

	// Check if API key is available
	apiKey := os.Getenv("ALPHAVANTAGE_API_KEY")
	if apiKey == "" {
		h.writeErrorResponse(w, http.StatusInternalServerError, "API key not configured")
		return
	}

	// Make API request with proper error handling
	stockData, err := h.fetchStockData(stockReq, apiKey)
	if err != nil {
		h.writeErrorResponse(w, http.StatusServiceUnavailable, "Failed to fetch stock data")
		return
	}

	// Return success response
	h.writeSuccessResponse(w, http.StatusOK, "Stock data retrieved successfully", stockData)
}

func (h *stockHandler) fetchStockData(req StockRequest, apiKey string) (interface{}, error) {
	baseURL := "https://www.alphavantage.co/query"
	httpReq, err := http.NewRequest("GET", baseURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add query params
	q := httpReq.URL.Query()
	q.Add("function", req.Function)
	q.Add("symbol", req.Symbol)
	q.Add("interval", req.Interval)
	q.Add("apikey", apiKey)
	httpReq.URL.RawQuery = q.Encode()
	httpReq.Header.Set("Accept", "application/json")

	// Send request
	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Parse response
	//TODO: parse response based on Alpha Vantage response format
	var apiResponse map[string]interface{}
	if err := json.Unmarshal(body, &apiResponse); err != nil {
		return nil, fmt.Errorf("failed to parse API response: %w", err)
	}

	return apiResponse, nil
}

// UpdateOrCreateStock updates existing stock or creates new one if it doesn't exist
func (h *stockHandler) UpdateOrCreateStock(w http.ResponseWriter, r *http.Request) {
	var req UpdateStockRequest

	// Decode and validate request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid JSON format")
		return
	}

	// Validate required fields
	if req.Symbol == "" {
		h.writeErrorResponse(w, http.StatusBadRequest, "Symbol is required")
		return
	}
	if req.Price <= 0 {
		h.writeErrorResponse(w, http.StatusBadRequest, "Price must be greater than 0")
		return
	}
	if req.Date == "" {
		h.writeErrorResponse(w, http.StatusBadRequest, "Date is required")
		return
	}

	// Parse date
	date, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid date format. Use YYYY-MM-DD")
		return
	}

	// Update or create stock in database
	err = h.stocks.UpdateOrCreateStockBySymbol(req.Symbol, req.Price, date)
	if err != nil {
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to update/create stock")
		return
	}

	// Return success response
	h.writeSuccessResponse(w, http.StatusOK, "Stock updated/created successfully", map[string]interface{}{
		"symbol": req.Symbol,
		"price":  req.Price,
		"date":   req.Date,
	})
}
