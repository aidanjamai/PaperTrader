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
			return cachedData, nil
		}
	}

	// Cache miss - fetch from external API
	if s.apiKey == "" {
		return nil, fmt.Errorf("API key not configured")
	}

	stockData, err := s.fetchStockData(symbol)
	if err != nil {
		return nil, err
	}

	// Cache the result in Redis
	if s.stockCache != nil {
		if err := s.stockCache.SetStock(stockData.Symbol, stockData.Date, stockData, 0); err != nil {
			log.Printf("GetStock: failed to cache result in Redis: %v", err)
		}
	}

	return stockData, nil
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
func (s *MarketService) GetHistoricalData(symbol string) (*HistoricalData, error) {
	symbol, err := validateSymbol(symbol)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	twoDaysAgo := now.AddDate(0, 0, -2)
	yesterday := now.AddDate(0, 0, -1)
	startDate := twoDaysAgo.Format(DateLayoutAPI)
	endDate := yesterday.Format(DateLayoutAPI)

	// Check Redis cache first
	if s.historicalCache != nil {
		cachedData, err := s.historicalCache.GetHistorical(symbol, startDate, endDate)
		if err == nil && cachedData != nil {
			return cachedData, nil
		}
	}

	// Cache miss - fetch from API
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

	return &StockData{
		Symbol: entry.Symbol,
		Price:  entry.Close,
		Date:   parsedDate.Format(DateLayoutUS),
	}, nil
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
		return nil, fmt.Errorf("insufficient data")
	}

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

	// Cache in Redis
	if s.historicalCache != nil {
		if err := s.historicalCache.SetHistorical(symbol, startDate, endDate, response, 0); err != nil {
			log.Printf("fetchHistoricalStockData: failed to cache result in Redis: %v", err)
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
