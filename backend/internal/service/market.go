package service

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const (
	MarketStackTimeout = 30 * time.Second
	DateLayoutUS       = "01/02/2006"
	DateLayoutAPI      = "2006-01-02"
	APIDateLayout      = "2006-01-02T15:04:05+0000"
	MaxSymbolLength    = 10
	MinPrice           = 0.01
)

type MarketService struct {
	apiKey          string
	stockCache      StockCache
	historicalCache HistoricalCache
}

func NewMarketService(apiKey string, stockCache StockCache, historicalCache HistoricalCache) *MarketService {
	return &MarketService{
		apiKey:          apiKey,
		stockCache:      stockCache,
		historicalCache: historicalCache,
	}
}

// DTOs for Service Layer
type StockData struct {
	Symbol string  `json:"symbol"`
	Date   string  `json:"date"`
	Price  float64 `json:"price"`
}

type HistoricalData struct {
	Symbol           string  `json:"symbol"`
	Date             string  `json:"date"`
	PreviousPrice    float64 `json:"previous_price"`
	Price            float64 `json:"price"`
	Volume           int     `json:"volume"`
	Change           float64 `json:"change"`
	ChangePercentage float64 `json:"change_percentage"`
}

// GetStock retrieves stock data by symbol
func (s *MarketService) GetStock(symbol string) (*StockData, error) {
	symbol = sanitizeString(symbol)
	symbol, err := validateSymbol(symbol)
	if err != nil {
		return nil, err
	}

	today := time.Now().Format(DateLayoutUS)

	// Check Redis cache first
	if s.stockCache != nil {
		cachedData, err := s.stockCache.GetStock(symbol, today)
		if err == nil && cachedData != nil {
			log.Printf("[MarketService] GetStock CACHE HIT for %s (date: %s)", symbol, today)
			return cachedData, nil
		}
		log.Printf("[MarketService] GetStock CACHE MISS for %s (date: %s) - fetching from MarketStack API", symbol, today)
	} else {
		log.Printf("[MarketService] GetStock CACHE UNAVAILABLE for %s - fetching from MarketStack API", symbol)
	}

	// Cache miss - fetch from external API
	if s.apiKey == "" {
		return nil, fmt.Errorf("API key not configured")
	}

	log.Printf("[MarketService] GetStock API CALL to MarketStack for %s", symbol)
	stockData, err := s.fetchStockData(symbol)
	if err != nil {
		log.Printf("[MarketService] GetStock API CALL FAILED for %s: %v", symbol, err)
		return nil, err
	}

	// Cache the result in Redis
	if s.stockCache != nil {
		if err := s.stockCache.SetStock(stockData.Symbol, stockData.Date, stockData, 0); err != nil {
			log.Printf("[MarketService] GetStock: failed to cache result in Redis: %v", err)
		} else {
			log.Printf("[MarketService] GetStock CACHED result for %s (date: %s, price: $%.2f)", symbol, stockData.Date, stockData.Price)
		}
	}

	return stockData, nil
}

// GetBatchHistoricalData retrieves historical data for multiple symbols in a single request
// This is more efficient than making individual requests for each symbol
func (s *MarketService) GetBatchHistoricalData(symbols []string) (map[string]*HistoricalData, error) {
	if len(symbols) == 0 {
		return make(map[string]*HistoricalData), nil
	}

	// Validate all symbols first
	validatedSymbols := make([]string, 0, len(symbols))
	for _, symbol := range symbols {
		validated, err := validateSymbol(symbol)
		if err != nil {
			log.Printf("[MarketService] GetBatchHistoricalData: invalid symbol %s: %v", symbol, err)
			continue
		}
		validatedSymbols = append(validatedSymbols, validated)
	}

	if len(validatedSymbols) == 0 {
		return nil, fmt.Errorf("no valid symbols provided")
	}

	now := time.Now()
	sevenDaysAgo := now.AddDate(0, 0, -7)
	yesterday := now.AddDate(0, 0, -1)
	startDate := sevenDaysAgo.Format(DateLayoutAPI)
	endDate := yesterday.Format(DateLayoutAPI)

	// Check cache for all symbols first
	result := make(map[string]*HistoricalData)
	symbolsToFetch := make([]string, 0)

	for _, symbol := range validatedSymbols {
		if s.historicalCache != nil {
			cachedData, err := s.historicalCache.GetHistorical(symbol, startDate, endDate)
			if err == nil && cachedData != nil {
				log.Printf("[MarketService] GetBatchHistoricalData CACHE HIT for %s", symbol)
				result[symbol] = cachedData
				continue
			}
		}
		symbolsToFetch = append(symbolsToFetch, symbol)
	}

	// If all symbols were cached, return early
	if len(symbolsToFetch) == 0 {
		log.Printf("[MarketService] GetBatchHistoricalData: all %d symbols served from cache", len(validatedSymbols))
		return result, nil
	}

	log.Printf("[MarketService] GetBatchHistoricalData: fetching %d symbols from API (cached: %d)", len(symbolsToFetch), len(validatedSymbols)-len(symbolsToFetch))

	// Fetch remaining symbols from API in batches (MarketStack supports up to 5 symbols per request on free tier)
	// For production, you might want to check your MarketStack plan limits
	const batchSize = 5
	for i := 0; i < len(symbolsToFetch); i += batchSize {
		end := i + batchSize
		if end > len(symbolsToFetch) {
			end = len(symbolsToFetch)
		}
		batch := symbolsToFetch[i:end]

		batchData, err := s.fetchBatchHistoricalStockData(batch, startDate, endDate)
		if err != nil {
			log.Printf("[MarketService] GetBatchHistoricalData: batch fetch failed for symbols %v: %v", batch, err)
			// Continue with other batches even if one fails
			continue
		}

		// Add batch results to main result map and cache them
		for symbol, data := range batchData {
			result[symbol] = data
			if s.historicalCache != nil {
				if err := s.historicalCache.SetHistorical(symbol, startDate, endDate, data, 0); err != nil {
					log.Printf("[MarketService] GetBatchHistoricalData: failed to cache %s: %v", symbol, err)
				}
			}
		}
	}

	log.Printf("[MarketService] GetBatchHistoricalData: completed, returning %d symbols", len(result))
	return result, nil
}

// fetchBatchHistoricalStockData fetches historical data for multiple symbols in one API call
func (s *MarketService) fetchBatchHistoricalStockData(symbols []string, startDate, endDate string) (map[string]*HistoricalData, error) {
	if s.apiKey == "" {
		return nil, fmt.Errorf("API key not configured")
	}

	const baseURL = "https://api.marketstack.com/v1/eod"
	httpReq, err := http.NewRequest("GET", baseURL, nil)
	if err != nil {
		return nil, err
	}

	q := httpReq.URL.Query()
	// MarketStack supports comma-separated symbols
	q.Add("symbols", strings.Join(symbols, ","))
	q.Add("date_from", startDate)
	q.Add("date_to", endDate)
	q.Add("access_key", s.apiKey)
	httpReq.URL.RawQuery = q.Encode()

	client := &http.Client{Timeout: MarketStackTimeout}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var apiResp struct {
		Data []struct {
			Symbol string  `json:"symbol"`
			Date   string  `json:"date"`
			Close  float64 `json:"close"`
			Volume float64 `json:"volume"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, err
	}

	if len(apiResp.Data) == 0 {
		return nil, fmt.Errorf("no data returned from API")
	}

	// Group data by symbol
	// MarketStack returns data sorted by date (most recent first) for all symbols
	symbolData := make(map[string][]struct {
		Date   string
		Close  float64
		Volume float64
	})

	for _, entry := range apiResp.Data {
		symbolData[entry.Symbol] = append(symbolData[entry.Symbol], struct {
			Date   string
			Close  float64
			Volume float64
		}{
			Date:   entry.Date,
			Close:  entry.Close,
			Volume: entry.Volume,
		})
	}

	// Process each symbol's data
	result := make(map[string]*HistoricalData)
	for _, symbol := range symbols {
		data, exists := symbolData[symbol]
		if !exists || len(data) < 2 {
			log.Printf("[MarketService] fetchBatchHistoricalStockData: insufficient data for %s (got %d days)", symbol, len(data))
			continue
		}

		// MarketStack returns data sorted by date (most recent first)
		// Use first 2 entries (latest and previous trading days)
		latest := data[0]
		previous := data[1]

		priceChange := latest.Close - previous.Close
		changePercent := (priceChange / previous.Close) * 100

		parsedDate, err := time.Parse(APIDateLayout, latest.Date)
		if err != nil {
			log.Printf("[MarketService] fetchBatchHistoricalStockData: failed to parse date for %s: %v", symbol, err)
			continue
		}

		result[symbol] = &HistoricalData{
			Symbol:           symbol,
			Date:             parsedDate.Format(DateLayoutUS),
			PreviousPrice:    previous.Close,
			Price:            latest.Close,
			Volume:           int(latest.Volume),
			Change:           math.Round(priceChange*100) / 100,
			ChangePercentage: math.Round(changePercent*100) / 100,
		}

		log.Printf("[MarketService] fetchBatchHistoricalStockData: processed %s (price: $%.2f, change: $%.2f, change%%: %.2f%%)", symbol, result[symbol].Price, result[symbol].Change, result[symbol].ChangePercentage)
	}

	return result, nil
}

func (s *MarketService) SaveStock(symbol string, price float64) error {
	symbol = sanitizeString(symbol)
	symbol, err := validateSymbol(symbol)
	if err != nil {
		return err
	}
	if err := validatePrice(price); err != nil {
		return err
	}

	today := time.Now().Format(DateLayoutUS)

	// Invalidate Redis cache for this symbol
	if s.stockCache != nil {
		s.stockCache.InvalidateStock(symbol)
}

	// Update Redis cache with new price
	if s.stockCache != nil {
		stockData := &StockData{
			Symbol: symbol,
			Price:  price,
			Date:   today,
		}
		if err := s.stockCache.SetStock(symbol, today, stockData, 0); err != nil {
			log.Printf("SaveStock: failed to update Redis cache: %v", err)
		}
	}

	return nil
}

func (s *MarketService) DeleteStockBySymbol(symbol string) error {
	// Invalidate Redis cache for this symbol (force refresh from API on next request)
	if s.stockCache != nil {
		return s.stockCache.InvalidateStock(symbol)
}
	return nil
}

// GetHistoricalData retrieves historical data
// Requests last 7 days to ensure we get at least 2 trading days (accounting for weekends/holidays)
func (s *MarketService) GetHistoricalData(symbol string) (*HistoricalData, error) {
	symbol, err := validateSymbol(symbol)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	// Request last 7 days to ensure we get at least 2 trading days even over weekends/holidays
	sevenDaysAgo := now.AddDate(0, 0, -7)
	yesterday := now.AddDate(0, 0, -1)
	startDate := sevenDaysAgo.Format(DateLayoutAPI)
	endDate := yesterday.Format(DateLayoutAPI)

	// Check Redis cache first
	// Use a cache key that represents "recent historical data" (not date-specific)
	// This allows cache hits even if the exact date range varies slightly
	if s.historicalCache != nil {
		cachedData, err := s.historicalCache.GetHistorical(symbol, startDate, endDate)
		if err == nil && cachedData != nil {
			log.Printf("[MarketService] GetHistoricalData CACHE HIT for %s (range: %s to %s)", symbol, startDate, endDate)
			return cachedData, nil
		}
		log.Printf("[MarketService] GetHistoricalData CACHE MISS for %s (range: %s to %s) - fetching from MarketStack API", symbol, startDate, endDate)
	} else {
		log.Printf("[MarketService] GetHistoricalData CACHE UNAVAILABLE for %s - fetching from MarketStack API", symbol)
	}

	// Cache miss - fetch from API
	log.Printf("[MarketService] GetHistoricalData API CALL to MarketStack for %s (range: %s to %s)", symbol, startDate, endDate)
	return s.fetchHistoricalStockData(symbol, startDate, endDate)
}

// Private helpers
func (s *MarketService) fetchStockData(symbol string) (*StockData, error) {
	const baseURL = "https://api.marketstack.com/v1/eod/latest"
	httpReq, err := http.NewRequest("GET", baseURL, nil)
	if err != nil {
		return nil, err
	}

	q := httpReq.URL.Query()
	q.Add("symbols", symbol)
	q.Add("access_key", s.apiKey)
	httpReq.URL.RawQuery = q.Encode()
	httpReq.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: MarketStackTimeout}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var apiResp struct {
		Data []struct {
			Close  float64 `json:"close"`
			Symbol string  `json:"symbol"`
			Date   string  `json:"date"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, err
	}

	if len(apiResp.Data) == 0 {
		return nil, fmt.Errorf("no data found")
	}

	entry := apiResp.Data[0]
	parsedDate, _ := time.Parse(time.RFC3339, entry.Date) // simplified error handling for brevity

	stockData := &StockData{
		Symbol: entry.Symbol,
		Price:  entry.Close,
		Date:   parsedDate.Format(DateLayoutUS),
	}
	
	log.Printf("[MarketService] GetStock API CALL SUCCESS for %s from MarketStack (price: $%.2f, date: %s)", symbol, stockData.Price, stockData.Date)
	return stockData, nil
}

func (s *MarketService) fetchHistoricalStockData(symbol, startDate, endDate string) (*HistoricalData, error) {
	const baseURL = "https://api.marketstack.com/v1/eod"
	httpReq, err := http.NewRequest("GET", baseURL, nil)
	if err != nil {
		return nil, err
	}

	q := httpReq.URL.Query()
	q.Add("symbols", symbol)
	q.Add("date_from", startDate)
	q.Add("date_to", endDate)
	q.Add("access_key", s.apiKey)
	httpReq.URL.RawQuery = q.Encode()

	client := &http.Client{Timeout: MarketStackTimeout}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var apiResp struct {
		Data []struct {
			Date   string  `json:"date"`
			Close  float64 `json:"close"`
			Volume float64 `json:"volume"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, err
	}

	if len(apiResp.Data) < 2 {
		log.Printf("[MarketService] GetHistoricalData API CALL FAILED for %s: insufficient data (got %d days, need 2)", symbol, len(apiResp.Data))
		return nil, fmt.Errorf("insufficient data")
	}

	// MarketStack returns data sorted by date (most recent first)
	// Use the first 2 entries (latest and previous trading days)
	// This works even if there were weekends/holidays in the date range
	latest := apiResp.Data[0]
	previous := apiResp.Data[1]
	priceChange := latest.Close - previous.Close
	changePercent := (priceChange / previous.Close) * 100

	parsedDate, _ := time.Parse(APIDateLayout, latest.Date)

	response := &HistoricalData{
		Symbol:           symbol,
		Date:             parsedDate.Format(DateLayoutUS),
		PreviousPrice:    previous.Close,
		Price:            latest.Close,
		Volume:           int(latest.Volume),
		Change:           math.Round(priceChange*100) / 100,
		ChangePercentage: math.Round(changePercent*100) / 100,
	}
	
	log.Printf("[MarketService] GetHistoricalData API CALL SUCCESS for %s from MarketStack (price: $%.2f, change: $%.2f, change%%: %.2f%%, got %d trading days)", symbol, response.Price, response.Change, response.ChangePercentage, len(apiResp.Data))

	// Cache in Redis
	if s.historicalCache != nil {
		if err := s.historicalCache.SetHistorical(symbol, startDate, endDate, response, 0); err != nil {
			log.Printf("[MarketService] fetchHistoricalStockData: failed to cache result in Redis: %v", err)
		} else {
			log.Printf("[MarketService] GetHistoricalData CACHED result for %s (price: $%.2f, change: $%.2f, change%%: %.2f%%)", symbol, response.Price, response.Change, response.ChangePercentage)
		}
	}

	return response, nil
}

// Helpers
var symbolRegex = regexp.MustCompile(`^[A-Z]{1,10}(\.[A-Z]{1,2})?$`)

func validateSymbol(symbol string) (string, error) {
	symbol = strings.TrimSpace(strings.ToUpper(symbol))
	if !symbolRegex.MatchString(symbol) {
		return "", fmt.Errorf("invalid symbol")
	}
	return symbol, nil
}

func validatePrice(price float64) error {
	if price < MinPrice {
		return fmt.Errorf("price too low")
	}
	return nil
}

func sanitizeString(input string) string {
	return strings.TrimSpace(strings.ReplaceAll(input, "\x00", ""))
}
