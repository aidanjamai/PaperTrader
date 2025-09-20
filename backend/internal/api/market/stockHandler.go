package market

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
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

type StockHandler struct {
	stocks data.Stocks
}

func NewStockHandler(stocks data.Stocks) *StockHandler {
	return &StockHandler{stocks: stocks}
}

func (h *StockHandler) writeJSONResponse(w http.ResponseWriter, statusCode int, response interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}

func (h *StockHandler) writeSuccessResponse(w http.ResponseWriter, statusCode int, message string, data interface{}) {
	response := MarketResponse{
		Success: true,
		Message: message,
		Data:    data,
	}
	h.writeJSONResponse(w, statusCode, response)
}

func (h *StockHandler) writeErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	response := ErrorResponse{
		Success: false,
		Message: message,
	}
	h.writeJSONResponse(w, statusCode, response)
}

func (h *StockHandler) validateStockRequest(req *StockSymbolRequest) error {
	if req.Symbol == "" {
		return fmt.Errorf("symbol is required")
	}
	return nil
}

// Handler methods
func (h *StockHandler) GetStock(w http.ResponseWriter, r *http.Request) {
	// Extract symbol from query parameters for GET request
	symbol := r.URL.Query().Get("symbol")
	log.Printf("Received stock request for symbol: %s", symbol)

	date := time.Now()
	dateString := date.Format("01/02/2006")

	// Validate required fields
	if symbol == "" {
		log.Printf("GetStock: Error - symbol is required")
		h.writeErrorResponse(w, http.StatusBadRequest, "Symbol parameter is required")
		return
	}

	// Create StockSymbolRequest for compatibility with existing methods
	stockReq := StockSymbolRequest{Symbol: symbol}

	//check db if stock exists in db for todays date
	//if it does, return the stock and dont make api request
	stock, err := h.stocks.GetStockBySymbolAndDate(stockReq.Symbol, dateString)
	if err != nil {
		log.Printf("GetStock: Error getting stock data: %v", err)
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to get stock data")
		return
	}

	if stock != nil {
		log.Printf("Returning cached stock data for %s", stockReq.Symbol)
		h.writeSuccessResponse(w, http.StatusOK, "Stock data retrieved successfully", stock)
		return
	}

	log.Printf("Stock %s not found in database for date %s, fetching from API", stockReq.Symbol, dateString)
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
	log.Printf("Stock data fetched from Alpha Vantage for %s with price %f and date %s", stockData.Symbol, stockData.Price, stockData.Date)

	// Update or create stock in database
	err = h.stocks.UpdateOrCreateStockBySymbol(stockData.Symbol, stockData.Price, stockData.Date)
	if err != nil {
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to update/create stock in db")
		return
	}
	log.Printf("Stock data updated/created in db for %s", stockData.Symbol)

	// Return success response
	h.writeSuccessResponse(w, http.StatusOK, "Stock data retrieved and updated/created in db successfully", stockData)
}

func (h *StockHandler) PostStock(w http.ResponseWriter, r *http.Request) {
	log.Printf("Received stock request: %+v", r.URL.Query())
	var stockReq PostStockRequest
	date := time.Now()
	dateString := date.Format("01/02/2006")

	// Decode and validate request - THIS WAS MISSING!
	if err := json.NewDecoder(r.Body).Decode(&stockReq); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid JSON format")
		return
	}

	// Validate required fields
	if stockReq.Symbol == "" {
		h.writeErrorResponse(w, http.StatusBadRequest, "Symbol is required")
		return
	}

	if stockReq.Price <= 0 {
		h.writeErrorResponse(w, http.StatusBadRequest, "Price must be greater than 0")
		return
	}

	log.Printf("Processing stock request for %s with price %f", stockReq.Symbol, stockReq.Price)

	err := h.stocks.UpdateOrCreateStockBySymbol(stockReq.Symbol, stockReq.Price, dateString)
	if err != nil {
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to update/create stock in db")
		return
	}
	log.Printf("Stock data updated/created in db for %s", stockReq.Symbol)

	h.writeSuccessResponse(w, http.StatusOK, "Stock data updated/created in db successfully", stockReq)
}

func (h *StockHandler) GetAllStocks(w http.ResponseWriter, r *http.Request) {
	log.Printf("Received all stocks request")
	stocks, err := h.stocks.GetAllStocks()
	if err != nil {
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to get all stocks")
		return
	}
	h.writeSuccessResponse(w, http.StatusOK, "All stocks retrieved successfully", stocks)
}

func (h *StockHandler) fetchStockData(req StockSymbolRequest, apiKey string) (StockResponse, error) {
	baseURL := "https://www.alphavantage.co/query"
	httpReq, err := http.NewRequest("GET", baseURL, nil)
	if err != nil {
		return StockResponse{}, fmt.Errorf("failed to create request: %w", err)
	}

	// Add query params
	q := httpReq.URL.Query()
	q.Add("function", "GLOBAL_QUOTE")
	q.Add("symbol", req.Symbol)
	q.Add("apikey", apiKey)
	httpReq.URL.RawQuery = q.Encode()
	httpReq.Header.Set("Accept", "application/json")

	// Send request
	// Add timeout to HTTP client
	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return StockResponse{}, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return StockResponse{}, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return StockResponse{}, fmt.Errorf("failed to read response: %w", err)
	}

	// Parse response
	//TODO: parse response based on Alpha Vantage response format
	var responseMap map[string]interface{}
	if err := json.Unmarshal(body, &responseMap); err != nil {
		return StockResponse{}, fmt.Errorf("failed to parse API response: %w", err)
	}
	stockResponse, err := h.parseGlobalQuote(responseMap)
	if err != nil {
		return StockResponse{}, fmt.Errorf("failed to parse global quote: %w", err)
	}

	return stockResponse, nil
}

func (h *StockHandler) parseGlobalQuote(responseMap map[string]interface{}) (StockResponse, error) {
	globalQuote, exists := responseMap["Global Quote"]
	if !exists {
		return StockResponse{}, fmt.Errorf("Global Quote not found in response")
	}

	quoteMap, ok := globalQuote.(map[string]interface{})
	if !ok {
		return StockResponse{}, fmt.Errorf("invalid Global Quote format")
	}

	// Extract symbol
	symbolStr, exists := quoteMap["01. symbol"]
	if !exists {
		return StockResponse{}, fmt.Errorf("symbol not found in Global Quote")
	}

	symbol, ok := symbolStr.(string)
	if !ok {
		return StockResponse{}, fmt.Errorf("symbol is not a string")
	}

	// Extract price
	priceStr, exists := quoteMap["05. price"]
	if !exists {
		return StockResponse{}, fmt.Errorf("price not found in Global Quote")
	}

	price, ok := priceStr.(string)
	if !ok {
		return StockResponse{}, fmt.Errorf("price is not a string")
	}

	// Extract trading day
	dateStr, exists := quoteMap["07. latest trading day"]
	if !exists {
		return StockResponse{}, fmt.Errorf("latest trading day not found")
	}

	tradingDay, ok := dateStr.(string)
	if !ok {
		return StockResponse{}, fmt.Errorf("latest trading day is not a string")
	}

	// Convert price string to float64
	var priceFloat float64
	if _, err := fmt.Sscanf(price, "%f", &priceFloat); err != nil {
		return StockResponse{}, fmt.Errorf("failed to parse price: %v", err)
	}

	// Convert date from "2025-09-17" to "09/17/2025" format
	parsedDate, err := time.Parse("2006-01-02", tradingDay)
	if err != nil {
		return StockResponse{}, fmt.Errorf("failed to parse date: %v", err)
	}

	formattedDate := parsedDate.Format("01/02/2006")

	return StockResponse{
		Symbol: symbol,
		Price:  priceFloat,
		Date:   formattedDate,
	}, nil
}

func (h *StockHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Format(time.RFC3339),
		"service":   "stock-api",
	}
	h.writeSuccessResponse(w, http.StatusOK, "Service is healthy", response)
}

// Delete stock by id
func (h *StockHandler) DeleteStock(w http.ResponseWriter, r *http.Request) {
	log.Printf("Received delete stock request: %+v", r.URL.Query())
	var stockReq StockIdRequest

	if err := json.NewDecoder(r.Body).Decode(&stockReq); err != nil {
		log.Printf("DeleteStock: Error decoding JSON: %v", err)
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid JSON format")
		return
	}

	if stockReq.ID == "" {
		log.Printf("DeleteStock: Error - ID is required")
		h.writeErrorResponse(w, http.StatusBadRequest, "Stock ID is required")
		return
	}

	log.Printf("DeleteStock: Attempting to delete stock with ID=%s", stockReq.ID)

	err := h.stocks.DeleteStockById(stockReq.ID)
	if err != nil {
		log.Printf("DeleteStock: Error deleting stock: %v", err)
		if err.Error() == "stock not found" {
			h.writeErrorResponse(w, http.StatusNotFound, "Stock not found")
		} else {
			h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to delete stock")
		}
		return
	}

	log.Printf("DeleteStock: Successfully deleted stock with ID=%s", stockReq.ID)
	h.writeSuccessResponse(w, http.StatusOK, "Stock deleted successfully", nil)
}

// Delete stock by symbol
func (h *StockHandler) DeleteStockBySymbol(w http.ResponseWriter, r *http.Request) {
	log.Printf("Received delete stock by symbol request: %+v", r.URL.Query())
	var stockReq StockSymbolRequest

	if err := json.NewDecoder(r.Body).Decode(&stockReq); err != nil {
		log.Printf("DeleteStockBySymbol: Error decoding JSON: %v", err)
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid JSON format")
		return
	}

	if stockReq.Symbol == "" {
		log.Printf("DeleteStockBySymbol: Error - Symbol is required")
		h.writeErrorResponse(w, http.StatusBadRequest, "Stock symbol is required")
		return
	}

	log.Printf("DeleteStockBySymbol: Attempting to delete stock with Symbol=%s", stockReq.Symbol)

	err := h.stocks.DeleteStockBySymbol(stockReq.Symbol)
	if err != nil {
		log.Printf("DeleteStockBySymbol: Error deleting stock: %v", err)
		if err.Error() == "stock not found" {
			h.writeErrorResponse(w, http.StatusNotFound, "Stock not found")
		} else {
			h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to delete stock")
		}
		return
	}

	log.Printf("DeleteStockBySymbol: Successfully deleted stock with Symbol=%s", stockReq.Symbol)
	h.writeSuccessResponse(w, http.StatusOK, "Stock deleted successfully", nil)
}

// delete all stocks
func (h *StockHandler) DeleteAllStocks(w http.ResponseWriter, r *http.Request) {
	log.Printf("Received delete all stocks request: %+v", r.URL.Query())

	log.Printf("DeleteAllStocks: Attempting to delete all stocks")

	err := h.stocks.DeleteAllStocks()
	if err != nil {
		log.Printf("DeleteAllStocks: Error deleting all stocks: %v", err)
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to delete all stocks")
		return
	}

	log.Printf("DeleteAllStocks: Successfully deleted all stocks")
	h.writeSuccessResponse(w, http.StatusOK, "All stocks deleted successfully", nil)
}
