package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"papertrader/internal/util"
)

const (
	MarketStackTimeout = 30 * time.Second
	// DateLayoutUS is the user-facing date format (MM/DD/YYYY) we surface in API responses.
	DateLayoutUS = "01/02/2006"
	// DateLayoutISO is the wire format MarketStack expects on date_from / date_to query parameters.
	DateLayoutISO = "2006-01-02"
	// DateLayoutMarketStack is the timestamp format MarketStack returns on response payloads.
	DateLayoutMarketStack = "2006-01-02T15:04:05+0000"
	MaxSymbolLength       = 10
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
	Symbol string          `json:"symbol"`
	Date   string          `json:"date"`
	Price  decimal.Decimal `json:"price"`
}

type HistoricalData struct {
	Symbol           string          `json:"symbol"`
	Date             string          `json:"date"`
	PreviousPrice    decimal.Decimal `json:"previous_price"`
	Price            decimal.Decimal `json:"price"`
	Volume           int             `json:"volume"`
	Change           decimal.Decimal `json:"change"`
	ChangePercentage decimal.Decimal `json:"change_percentage"`
}

// GetStock retrieves stock data by symbol
func (s *MarketService) GetStock(ctx context.Context, symbol string) (*StockData, error) {
	symbol, err := util.ValidateSymbol(symbol)
	if err != nil {
		return nil, err
	}

	today := time.Now().Format(DateLayoutUS)

	// Check Redis cache first
	if s.stockCache != nil {
		cachedData, err := s.stockCache.GetStock(ctx, symbol, today)
		if err == nil && cachedData != nil {
			slog.Debug("stock cache hit", "symbol", symbol, "date", today)
			return cachedData, nil
		}
		slog.Debug("stock cache miss", "symbol", symbol, "date", today)
	} else {
		slog.Warn("stock cache unavailable", "symbol", symbol, "component", "market")
	}

	// Cache miss - fetch from external API
	if s.apiKey == "" {
		return nil, fmt.Errorf("API key not configured")
	}

	stockData, err := s.fetchStockData(ctx, symbol)
	if err != nil {
		slog.Warn("MarketStack API call failed for GetStock", "symbol", symbol, "err", err)
		return nil, err
	}

	// Cache the result in Redis
	if s.stockCache != nil {
		if err := s.stockCache.SetStock(ctx, stockData.Symbol, stockData.Date, stockData, 0); err != nil {
			slog.Warn("failed to cache stock result", "symbol", symbol, "err", err, "component", "market")
		}
	}

	return stockData, nil
}

// GetBatchHistoricalData retrieves historical data for multiple symbols in a single request
// This is more efficient than making individual requests for each symbol
func (s *MarketService) GetBatchHistoricalData(ctx context.Context, symbols []string) (map[string]*HistoricalData, error) {
	if len(symbols) == 0 {
		return make(map[string]*HistoricalData), nil
	}

	// Validate all symbols first
	validatedSymbols := make([]string, 0, len(symbols))
	for _, symbol := range symbols {
		validated, err := util.ValidateSymbol(symbol)
		if err != nil {
			slog.Debug("skipping invalid symbol in batch", "symbol", symbol, "err", err)
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
	startDate := sevenDaysAgo.Format(DateLayoutISO)
	endDate := yesterday.Format(DateLayoutISO)

	// Check cache for all symbols first
	result := make(map[string]*HistoricalData)
	symbolsToFetch := make([]string, 0)

	for _, symbol := range validatedSymbols {
		if s.historicalCache != nil {
			cachedData, err := s.historicalCache.GetHistorical(ctx, symbol, startDate, endDate)
			if err == nil && cachedData != nil {
				slog.Debug("historical cache hit", "symbol", symbol)
				result[symbol] = cachedData
				continue
			}
		}
		symbolsToFetch = append(symbolsToFetch, symbol)
	}

	// If all symbols were cached, return early
	if len(symbolsToFetch) == 0 {
		slog.Debug("all symbols served from historical cache", "count", len(validatedSymbols))
		return result, nil
	}

	slog.Debug("fetching symbols from MarketStack API",
		"fetch_count", len(symbolsToFetch),
		"cached_count", len(validatedSymbols)-len(symbolsToFetch),
	)

	// Fetch remaining symbols from API in batches (MarketStack supports up to 5 symbols per request on free tier)
	const batchSize = 5
	for i := 0; i < len(symbolsToFetch); i += batchSize {
		end := i + batchSize
		if end > len(symbolsToFetch) {
			end = len(symbolsToFetch)
		}
		batch := symbolsToFetch[i:end]

		batchData, err := s.fetchBatchHistoricalStockData(ctx, batch, startDate, endDate)
		if err != nil {
			slog.Warn("batch historical fetch failed", "symbols", batch, "err", err, "component", "market")
			// Continue with other batches even if one fails
			continue
		}

		// Add batch results to main result map and cache them
		for symbol, data := range batchData {
			result[symbol] = data
			if s.historicalCache != nil {
				if err := s.historicalCache.SetHistorical(ctx, symbol, startDate, endDate, data, 0); err != nil {
					slog.Warn("failed to cache historical result", "symbol", symbol, "err", err, "component", "market")
				}
			}
		}
	}

	slog.Debug("GetBatchHistoricalData completed", "returned_count", len(result))
	return result, nil
}

// fetchBatchHistoricalStockData fetches historical data for multiple symbols in one API call
func (s *MarketService) fetchBatchHistoricalStockData(ctx context.Context, symbols []string, startDate, endDate string) (map[string]*HistoricalData, error) {
	if s.apiKey == "" {
		return nil, fmt.Errorf("API key not configured")
	}

	const baseURL = "https://api.marketstack.com/v1/eod"
	httpReq, err := http.NewRequestWithContext(ctx, "GET", baseURL, nil)
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
			slog.Debug("insufficient data for symbol in batch", "symbol", symbol, "days_returned", len(data))
			continue
		}

		// MarketStack returns data sorted by date (most recent first)
		// Use first 2 entries (latest and previous trading days)
		latest := data[0]
		previous := data[1]

		// Convert from float64 (external API) to decimal at the boundary.
		// -2 exponent snaps to 2dp (stock prices are natively 2dp).
		latestDec := decimal.NewFromFloatWithExponent(latest.Close, -2)
		prevDec := decimal.NewFromFloatWithExponent(previous.Close, -2)

		priceChange := latestDec.Sub(prevDec)
		var changePercent decimal.Decimal
		if !prevDec.IsZero() {
			changePercent = priceChange.Div(prevDec).Mul(decimal.NewFromInt(100)).Round(2)
		}

		parsedDate, err := time.Parse(DateLayoutMarketStack, latest.Date)
		if err != nil {
			slog.Warn("failed to parse date for symbol", "symbol", symbol, "err", err, "component", "market")
			continue
		}

		result[symbol] = &HistoricalData{
			Symbol:           symbol,
			Date:             parsedDate.Format(DateLayoutUS),
			PreviousPrice:    prevDec,
			Price:            latestDec,
			Volume:           int(latest.Volume),
			Change:           priceChange.Round(2),
			ChangePercentage: changePercent,
		}
	}

	return result, nil
}

// GetHistoricalData retrieves historical data
// Requests last 7 days to ensure we get at least 2 trading days (accounting for weekends/holidays)
func (s *MarketService) GetHistoricalData(ctx context.Context, symbol string) (*HistoricalData, error) {
	symbol, err := util.ValidateSymbol(symbol)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	// Request last 7 days to ensure we get at least 2 trading days even over weekends/holidays
	sevenDaysAgo := now.AddDate(0, 0, -7)
	yesterday := now.AddDate(0, 0, -1)
	startDate := sevenDaysAgo.Format(DateLayoutISO)
	endDate := yesterday.Format(DateLayoutISO)

	// Check Redis cache first
	if s.historicalCache != nil {
		cachedData, err := s.historicalCache.GetHistorical(ctx, symbol, startDate, endDate)
		if err == nil && cachedData != nil {
			slog.Debug("historical cache hit", "symbol", symbol, "start_date", startDate, "end_date", endDate)
			return cachedData, nil
		}
		slog.Debug("historical cache miss", "symbol", symbol, "start_date", startDate, "end_date", endDate)
	} else {
		slog.Warn("historical cache unavailable", "symbol", symbol, "component", "market")
	}

	// Cache miss - fetch from API
	return s.fetchHistoricalStockData(ctx, symbol, startDate, endDate)
}

// Private helpers
func (s *MarketService) fetchStockData(ctx context.Context, symbol string) (*StockData, error) {
	const baseURL = "https://api.marketstack.com/v1/eod/latest"
	httpReq, err := http.NewRequestWithContext(ctx, "GET", baseURL, nil)
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
	parsedDate, err := time.Parse(DateLayoutMarketStack, entry.Date)
	if err != nil {
		return nil, fmt.Errorf("parse date %q: %w", entry.Date, err)
	}

	stockData := &StockData{
		Symbol: entry.Symbol,
		Price:  decimal.NewFromFloatWithExponent(entry.Close, -2),
		Date:   parsedDate.Format(DateLayoutUS),
	}

	slog.Info("MarketStack API call succeeded for GetStock", "symbol", symbol, "price", stockData.Price, "date", stockData.Date)
	return stockData, nil
}

func (s *MarketService) fetchHistoricalStockData(ctx context.Context, symbol, startDate, endDate string) (*HistoricalData, error) {
	const baseURL = "https://api.marketstack.com/v1/eod"
	httpReq, err := http.NewRequestWithContext(ctx, "GET", baseURL, nil)
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
		slog.Warn("insufficient historical data from MarketStack", "symbol", symbol, "days_returned", len(apiResp.Data), "days_needed", 2)
		return nil, &InsufficientHistoricalDataError{}
	}

	// MarketStack returns data sorted by date (most recent first)
	// Use the first 2 entries (latest and previous trading days)
	// This works even if there were weekends/holidays in the date range
	latest := apiResp.Data[0]
	previous := apiResp.Data[1]

	// Convert from float64 (external API) to decimal at the boundary.
	latestDec := decimal.NewFromFloatWithExponent(latest.Close, -2)
	prevDec := decimal.NewFromFloatWithExponent(previous.Close, -2)

	priceChange := latestDec.Sub(prevDec)
	var changePercent decimal.Decimal
	if !prevDec.IsZero() {
		changePercent = priceChange.Div(prevDec).Mul(decimal.NewFromInt(100)).Round(2)
	}

	parsedDate, err := time.Parse(DateLayoutMarketStack, latest.Date)
	if err != nil {
		return nil, fmt.Errorf("parse latest date %q: %w", latest.Date, err)
	}

	response := &HistoricalData{
		Symbol:           symbol,
		Date:             parsedDate.Format(DateLayoutUS),
		PreviousPrice:    prevDec,
		Price:            latestDec,
		Volume:           int(latest.Volume),
		Change:           priceChange.Round(2),
		ChangePercentage: changePercent,
	}

	slog.Info("MarketStack API call succeeded for GetHistoricalData",
		"symbol", symbol,
		"price", response.Price,
		"change", response.Change,
		"change_pct", response.ChangePercentage,
		"trading_days", len(apiResp.Data),
	)

	// Cache in Redis
	if s.historicalCache != nil {
		if err := s.historicalCache.SetHistorical(ctx, symbol, startDate, endDate, response, 0); err != nil {
			slog.Warn("failed to cache historical result", "symbol", symbol, "err", err, "component", "market")
		}
	}

	return response, nil
}
