package market

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"papertrader/internal/data"
	"papertrader/internal/data/collections"
)

// Constants
const (
	MarketStackTimeout = 30 * time.Second
	DateLayoutUS       = "01/02/2006"
	DateLayoutAPI      = "2006-01-02"
	APIDateLayout      = "2006-01-02T15:04:05+0000"
	MaxSymbolLength    = 10
	MinPrice           = 0.01
)

// Validation functions
var symbolRegex = regexp.MustCompile(`^[A-Z]{1,10}(\.[A-Z]{1,2})?$`)

// validateSymbol validates stock symbol format
func validateSymbol(symbol string) error {
	if symbol == "" {
		return fmt.Errorf("symbol is required")
	}

	symbol = strings.TrimSpace(strings.ToUpper(symbol))
	if len(symbol) > MaxSymbolLength {
		return fmt.Errorf("symbol too long (max %d characters)", MaxSymbolLength)
	}

	if !symbolRegex.MatchString(symbol) {
		return fmt.Errorf("invalid symbol format (must be uppercase letters, optionally with .suffix)")
	}

	return nil
}

// validatePrice validates price values
func validatePrice(price float64) error {
	if price < MinPrice {
		return fmt.Errorf("price must be at least %.2f", MinPrice)
	}
	if price > 1000000 {
		return fmt.Errorf("price too high (max 1,000,000)")
	}
	return nil
}

// validateDateRange validates date range for historical data
func validateDateRange(startDate, endDate string) error {
	if startDate == "" || endDate == "" {
		return fmt.Errorf("start_date and end_date are required")
	}

	start, err := time.Parse(DateLayoutAPI, startDate)
	if err != nil {
		return fmt.Errorf("invalid start_date format (use YYYY-MM-DD)")
	}

	end, err := time.Parse(DateLayoutAPI, endDate)
	if err != nil {
		return fmt.Errorf("invalid end_date format (use YYYY-MM-DD)")
	}

	if start.After(end) {
		return fmt.Errorf("start_date must be before end_date")
	}

	now := time.Now()
	if start.After(now) || end.After(now) {
		return fmt.Errorf("dates cannot be in the future")
	}

	// Check date range isn't too large (max 1 year)
	if end.Sub(start).Hours() > 24*365 {
		return fmt.Errorf("date range too large (max 1 year)")
	}

	return nil
}

// validateID validates ID format
func validateID(id string) error {
	if id == "" {
		return fmt.Errorf("id is required")
	}

	if len(id) > 50 {
		return fmt.Errorf("id too long")
	}

	// Basic validation - should contain only alphanumeric characters and hyphens
	for _, char := range id {
		if !((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') || char == '-') {
			return fmt.Errorf("invalid id format")
		}
	}

	return nil
}

// sanitizeString basic input sanitization
func sanitizeString(input string) string {
	return strings.TrimSpace(strings.ReplaceAll(input, "\x00", ""))
}

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
	stocks     data.Stocks
	intraDaily collections.IntraDailyStore
}

func NewStockHandler(stocks data.Stocks, intraDaily collections.IntraDailyStore) *StockHandler {
	return &StockHandler{stocks: stocks, intraDaily: intraDaily}
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

// validateStockSymbolRequest validates stock symbol requests
func (h *StockHandler) validateStockSymbolRequest(req *StockSymbolRequest) error {
	if req == nil {
		return fmt.Errorf("request cannot be nil")
	}

	req.Symbol = sanitizeString(req.Symbol)
	return validateSymbol(req.Symbol)
}

// =============================================================================
// Handler Methods
// =============================================================================

// GetStock retrieves stock data by symbol, checking cache first then API
func (h *StockHandler) GetStock(w http.ResponseWriter, r *http.Request) {
	symbol := sanitizeString(r.URL.Query().Get("symbol"))

	// Validate input
	if err := validateSymbol(symbol); err != nil {
		log.Printf("GetStock: validation error for symbol %s: %v", symbol, err)
		h.writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	today := time.Now().Format(DateLayoutUS)
	log.Printf("GetStock: fetching data for symbol %s on date %s", symbol, today)

	// Check database cache first
	stock, err := h.stocks.GetStockBySymbolAndDate(symbol, today)
	if err != nil {
		log.Printf("GetStock: database error: %v", err)
		h.writeErrorResponse(w, http.StatusInternalServerError, "Database error")
		return
	}

	if stock != nil {
		log.Printf("GetStock: returning cached data for %s", symbol)
		h.writeSuccessResponse(w, http.StatusOK, "Stock data retrieved from cache", stock)
		return
	}

	// Fetch from external API
	log.Printf("GetStock: cache miss for %s, fetching from API", symbol)
	apiKey := os.Getenv("MARKETSTACK_API_KEY")
	if apiKey == "" {
		log.Printf("GetStock: API key not configured")
		h.writeErrorResponse(w, http.StatusInternalServerError, "Service configuration error")
		return
	}

	stockData, err := h.fetchStockData(StockSymbolRequest{Symbol: symbol}, apiKey)
	if err != nil {
		log.Printf("GetStock: API error: %v", err)
		h.writeErrorResponse(w, http.StatusServiceUnavailable, "Failed to fetch stock data from external API")
		return
	}

	// Cache the result
	if err := h.stocks.UpdateOrCreateStockBySymbol(stockData.Symbol, stockData.Price, stockData.Date); err != nil {
		log.Printf("GetStock: failed to cache result: %v", err)
		// Don't fail the request, just log the error
	}

	log.Printf("GetStock: successfully retrieved and cached data for %s", symbol)
	h.writeSuccessResponse(w, http.StatusOK, "Stock data retrieved successfully", stockData)
}

// PostStock creates or updates stock data via POST request
func (h *StockHandler) PostStock(w http.ResponseWriter, r *http.Request) {
	var stockReq PostStockRequest

	// Decode and validate request
	if err := json.NewDecoder(r.Body).Decode(&stockReq); err != nil {
		log.Printf("PostStock: invalid JSON: %v", err)
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid JSON format")
		return
	}

	// Sanitize and validate inputs
	stockReq.Symbol = sanitizeString(stockReq.Symbol)

	if err := validateSymbol(stockReq.Symbol); err != nil {
		log.Printf("PostStock: symbol validation failed: %v", err)
		h.writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := validatePrice(stockReq.Price); err != nil {
		log.Printf("PostStock: price validation failed: %v", err)
		h.writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	today := time.Now().Format(DateLayoutUS)
	log.Printf("PostStock: creating/updating stock %s with price %.2f", stockReq.Symbol, stockReq.Price)

	// Update or create in database
	if err := h.stocks.UpdateOrCreateStockBySymbol(stockReq.Symbol, stockReq.Price, today); err != nil {
		log.Printf("PostStock: database error: %v", err)
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to save stock data")
		return
	}

	log.Printf("PostStock: successfully saved stock data for %s", stockReq.Symbol)
	h.writeSuccessResponse(w, http.StatusOK, "Stock data saved successfully", stockReq)
}

// GetAllStocks retrieves all stocks from the database
func (h *StockHandler) GetAllStocks(w http.ResponseWriter, r *http.Request) {
	log.Printf("GetAllStocks: retrieving all stocks")

	stocks, err := h.stocks.GetAllStocks()
	if err != nil {
		log.Printf("GetAllStocks: database error: %v", err)
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to retrieve stocks")
		return
	}

	log.Printf("GetAllStocks: retrieved %d stocks", len(stocks))
	h.writeSuccessResponse(w, http.StatusOK, "Stocks retrieved successfully", stocks)
}

// =============================================================================
// Utility Functions
// =============================================================================

// fetchStockData fetches current stock data from external API
func (h *StockHandler) fetchStockData(req StockSymbolRequest, apiKey string) (StockResponse, error) {
	log.Printf("fetchStockData: fetching current data for %s", req.Symbol)

	const baseURL = "https://api.marketstack.com/v1/eod/latest"
	httpReq, err := http.NewRequest("GET", baseURL, nil)
	if err != nil {
		return StockResponse{}, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set query parameters
	q := httpReq.URL.Query()
	q.Add("symbols", req.Symbol)
	q.Add("access_key", apiKey)
	httpReq.URL.RawQuery = q.Encode()
	httpReq.Header.Set("Accept", "application/json")

	// Make API request
	client := &http.Client{Timeout: MarketStackTimeout}
	resp, err := client.Do(httpReq)
	if err != nil {
		return StockResponse{}, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return StockResponse{}, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return StockResponse{}, fmt.Errorf("failed to read API response: %w", err)
	}

	var apiResp struct {
		Data []struct {
			Open   float64 `json:"open"`
			High   float64 `json:"high"`
			Low    float64 `json:"low"`
			Close  float64 `json:"close"`
			Symbol string  `json:"symbol"`
			Date   string  `json:"date"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &apiResp); err != nil {
		return StockResponse{}, fmt.Errorf("failed to parse API response: %w", err)
	}

	if len(apiResp.Data) == 0 {
		return StockResponse{}, fmt.Errorf("API response contained no data")
	}

	entry := apiResp.Data[0]

	// Parse date with fallback formats
	parsedDate, err := time.Parse(time.RFC3339, entry.Date)
	if err != nil {
		parsedDate, err = time.Parse(DateLayoutAPI, entry.Date)
		if err != nil {
			return StockResponse{}, fmt.Errorf("failed to parse date %q: %w", entry.Date, err)
		}
	}

	return StockResponse{
		Symbol: entry.Symbol,
		Price:  entry.Close,
		Date:   parsedDate.Format(DateLayoutUS),
	}, nil
}

// HealthCheck provides service health status
func (h *StockHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Format(time.RFC3339),
		"service":   "stock-api",
		"version":   "1.0.0",
	}
	h.writeSuccessResponse(w, http.StatusOK, "Service is healthy", response)
}

// =============================================================================
// Delete Operations
// =============================================================================

// DeleteStock deletes a stock by ID
func (h *StockHandler) DeleteStock(w http.ResponseWriter, r *http.Request) {
	var stockReq StockIdRequest

	// Decode and validate request
	if err := json.NewDecoder(r.Body).Decode(&stockReq); err != nil {
		log.Printf("DeleteStock: invalid JSON: %v", err)
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid JSON format")
		return
	}

	// Validate ID
	stockReq.ID = sanitizeString(stockReq.ID)
	if err := validateID(stockReq.ID); err != nil {
		log.Printf("DeleteStock: validation error: %v", err)
		h.writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	log.Printf("DeleteStock: deleting stock with ID %s", stockReq.ID)

	// Attempt deletion
	err := h.stocks.DeleteStockById(stockReq.ID)
	if err != nil {
		log.Printf("DeleteStock: database error: %v", err)
		if strings.Contains(err.Error(), "not found") {
			h.writeErrorResponse(w, http.StatusNotFound, "Stock not found")
		} else {
			h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to delete stock")
		}
		return
	}

	log.Printf("DeleteStock: successfully deleted stock with ID %s", stockReq.ID)
	h.writeSuccessResponse(w, http.StatusOK, "Stock deleted successfully", nil)
}

// DeleteStockBySymbol deletes all stocks with a specific symbol
func (h *StockHandler) DeleteStockBySymbol(w http.ResponseWriter, r *http.Request) {
	var stockReq StockSymbolRequest

	// Decode and validate request
	if err := json.NewDecoder(r.Body).Decode(&stockReq); err != nil {
		log.Printf("DeleteStockBySymbol: invalid JSON: %v", err)
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid JSON format")
		return
	}

	// Validate symbol
	stockReq.Symbol = sanitizeString(stockReq.Symbol)
	if err := validateSymbol(stockReq.Symbol); err != nil {
		log.Printf("DeleteStockBySymbol: validation error: %v", err)
		h.writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	log.Printf("DeleteStockBySymbol: deleting stocks with symbol %s", stockReq.Symbol)

	// Attempt deletion
	err := h.stocks.DeleteStockBySymbol(stockReq.Symbol)
	if err != nil {
		log.Printf("DeleteStockBySymbol: database error: %v", err)
		if strings.Contains(err.Error(), "not found") {
			h.writeErrorResponse(w, http.StatusNotFound, "Stock not found")
		} else {
			h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to delete stocks")
		}
		return
	}

	log.Printf("DeleteStockBySymbol: successfully deleted stocks with symbol %s", stockReq.Symbol)
	h.writeSuccessResponse(w, http.StatusOK, "Stocks deleted successfully", nil)
}

// DeleteAllStocks deletes all stocks from the database
func (h *StockHandler) DeleteAllStocks(w http.ResponseWriter, r *http.Request) {
	log.Printf("DeleteAllStocks: deleting all stocks")

	err := h.stocks.DeleteAllStocks()
	if err != nil {
		log.Printf("DeleteAllStocks: database error: %v", err)
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to delete all stocks")
		return
	}

	log.Printf("DeleteAllStocks: successfully deleted all stocks")
	h.writeSuccessResponse(w, http.StatusOK, "All stocks deleted successfully", nil)
}

// =============================================================================
// Historical Data Operations
// =============================================================================

// GetStockHistoricalDataDaily retrieves historical stock data for the last 2 days
func (h *StockHandler) GetStockHistoricalDataDaily(w http.ResponseWriter, r *http.Request) {
	symbol := sanitizeString(r.URL.Query().Get("symbol"))

	// Validate symbol
	if err := validateSymbol(symbol); err != nil {
		log.Printf("GetStockHistoricalDataDaily: symbol validation error: %v", err)
		h.writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	// Auto-calculate date range: 2 days ago to yesterday
	now := time.Now()
	twoDaysAgo := now.AddDate(0, 0, -2)
	yesterday := now.AddDate(0, 0, -1)

	req := StockHistoricalDataDailyRequest{
		Symbol:    symbol,
		StartDate: twoDaysAgo.Format(DateLayoutAPI),
		EndDate:   yesterday.Format(DateLayoutAPI),
	}

	log.Printf("GetStockHistoricalDataDaily: fetching data for %s from %s to %s",
		req.Symbol, req.StartDate, req.EndDate)

	// Check API key
	apiKey := os.Getenv("MARKETSTACK_API_KEY")
	if apiKey == "" {
		log.Printf("GetStockHistoricalDataDaily: API key not configured")
		h.writeErrorResponse(w, http.StatusInternalServerError, "Service configuration error")
		return
	}

	// Fetch historical data
	historicalData, err := h.fetchHistoricalStockData(req, apiKey)
	if err != nil {
		log.Printf("GetStockHistoricalDataDaily: failed to fetch data: %v", err)
		h.writeErrorResponse(w, http.StatusServiceUnavailable, "Failed to fetch historical stock data")
		return
	}

	log.Printf("GetStockHistoricalDataDaily: successfully retrieved data for %s", symbol)
	h.writeSuccessResponse(w, http.StatusOK, "Historical stock data retrieved successfully", historicalData)
}

// fetchHistoricalStockData fetches historical stock data with caching support
func (h *StockHandler) fetchHistoricalStockData(req StockHistoricalDataDailyRequest, apiKey string) (StockHistoricalDataDailyResponse, error) {
	log.Printf("fetchHistoricalStockData: fetching data for %s (%s to %s)",
		req.Symbol, req.StartDate, req.EndDate)

	// Check MongoDB cache first
	intraDailyRequest := &collections.IntraDailyRequest{
		Symbol:    req.Symbol,
		StartDate: req.StartDate,
		EndDate:   req.EndDate,
	}

	cachedResponse, err := h.intraDaily.GetIntraDailyByRequest(intraDailyRequest)
	if err != nil && err != collections.ErrIntraDailyNotFound {
		// Cache lookup failed (not a "not found" error), log and continue to API
		log.Printf("fetchHistoricalStockData: cache lookup failed, proceeding to API: %v", err)
		cachedResponse = nil // Ensure we don't use potentially invalid cached data
	}

	// Return cached data if available
	if cachedResponse != nil {
		log.Printf("fetchHistoricalStockData: returning cached data for %s", req.Symbol)
		return StockHistoricalDataDailyResponse{
			Symbol:           cachedResponse.Symbol,
			Date:             cachedResponse.Date,
			PreviousPrice:    cachedResponse.PreviousPrice,
			Price:            cachedResponse.Price,
			Volume:           cachedResponse.Volume,
			Change:           cachedResponse.Change,
			ChangePercentage: cachedResponse.ChangePercentage,
		}, nil
	}

	// Cache miss - fetch from external API
	log.Printf("fetchHistoricalStockData: cache miss, fetching from API for %s", req.Symbol)
	const baseURL = "https://api.marketstack.com/v1/eod"
	httpReq, err := http.NewRequest("GET", baseURL, nil)
	if err != nil {
		return StockHistoricalDataDailyResponse{}, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set query parameters
	q := httpReq.URL.Query()
	q.Add("symbols", req.Symbol)
	q.Add("date_from", req.StartDate)
	q.Add("date_to", req.EndDate)
	q.Add("access_key", apiKey)
	httpReq.URL.RawQuery = q.Encode()
	httpReq.Header.Set("Accept", "application/json")

	// Make API request
	client := &http.Client{Timeout: MarketStackTimeout}
	resp, err := client.Do(httpReq)
	if err != nil {
		return StockHistoricalDataDailyResponse{}, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return StockHistoricalDataDailyResponse{}, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return StockHistoricalDataDailyResponse{}, fmt.Errorf("failed to read API response: %w", err)
	}

	var apiResp struct {
		Data []struct {
			Date   string  `json:"date"`
			Open   float64 `json:"open"`
			Close  float64 `json:"close"`
			Volume float64 `json:"volume"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &apiResp); err != nil {
		return StockHistoricalDataDailyResponse{}, fmt.Errorf("failed to parse API response: %w", err)
	}

	if len(apiResp.Data) < 2 {
		return StockHistoricalDataDailyResponse{}, fmt.Errorf("insufficient data: need at least 2 days of trading data")
	}

	// Process data (API returns latest first)
	latest := apiResp.Data[0]
	previous := apiResp.Data[1]

	// Calculate price change and percentage
	priceChange := latest.Close - previous.Close
	changePercent := (priceChange / previous.Close) * 100

	// Round values for consistency
	roundedChange := math.Round(priceChange*100) / 100
	roundedPercent := math.Round(changePercent*100) / 100

	// Parse and format date
	parsedDate, err := time.Parse(APIDateLayout, latest.Date)
	if err != nil {
		return StockHistoricalDataDailyResponse{}, fmt.Errorf("failed to parse API date: %w", err)
	}

	response := StockHistoricalDataDailyResponse{
		Symbol:           req.Symbol,
		Date:             parsedDate.Format(DateLayoutUS),
		PreviousPrice:    previous.Close,
		Price:            latest.Close,
		Volume:           int(latest.Volume),
		Change:           roundedChange,
		ChangePercentage: roundedPercent,
	}

	// Cache the response in MongoDB
	intraDailyResponse := &collections.IntraDailyResponse{
		Symbol:           response.Symbol,
		Date:             response.Date,
		PreviousPrice:    response.PreviousPrice,
		Price:            response.Price,
		Volume:           response.Volume,
		Change:           response.Change,
		ChangePercentage: response.ChangePercentage,
	}

	if _, err := h.intraDaily.CreateIntraDaily(intraDailyRequest, intraDailyResponse); err != nil {
		log.Printf("fetchHistoricalStockData: failed to cache response: %v", err)
		// Don't fail the request for caching errors
	} else {
		log.Printf("fetchHistoricalStockData: cached response for %s", req.Symbol)
	}

	return response, nil

}
