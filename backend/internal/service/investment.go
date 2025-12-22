package service

import (
	"database/sql"
	"errors"
	"math"
	"time"

	"github.com/google/uuid"

	"papertrader/internal/data"
)

type InvestmentService struct {
	db             *sql.DB
	marketService  *MarketService
	portfolioStore *data.PortfolioStore
}

func NewInvestmentService(db *sql.DB, marketService *MarketService, portfolioStore *data.PortfolioStore) *InvestmentService {
	return &InvestmentService{
		db:             db,
		marketService:  marketService,
		portfolioStore: portfolioStore,
	}
}

// Helper to round float to 2 decimal places
func round(val float64) float64 {
	return math.Round(val*100) / 100
}

func (s *InvestmentService) BuyStock(userID string, symbol string, quantity int) (*data.UserStock, error) {
	// 1. Get Stock Price from MarketService (Redis-backed)
	stockData, err := s.marketService.GetStock(symbol)
	if err != nil {
		return nil, err
	}
	price := stockData.Price
	totalPrice := round(price * float64(quantity))

	// 2. Start PostgreSQL Transaction (ACID - all operations atomic)
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	userStoreTx := data.NewUserStore(tx)
	tradeStoreTx := data.NewTradesStore(tx)
	portfolioStoreTx := data.NewPortfolioStore(tx)

	// 3. Get User and Validate Balance
	user, err := userStoreTx.GetUserByID(userID)
	if err != nil {
		return nil, err
	}

	if user.Balance < totalPrice {
		return nil, errors.New("insufficient funds")
	}

	// 4. Deduct Balance
	newBalance := user.Balance - totalPrice
	if err := userStoreTx.UpdateBalance(userID, newBalance); err != nil {
		return nil, err
	}

	// 5. Create Trade
	dateString := time.Now().Format("01/02/2006")
	trade := &data.Trade{
		ID:       uuid.New().String(),
		UserID:   userID,
		Symbol:   symbol,
		Action:   "BUY",
		Quantity: quantity,
		Price:    price,
		Date:     dateString,
		Status:   "COMPLETED",
	}

	if err := tradeStoreTx.CreateTrade(trade); err != nil {
		return nil, err
	}

	// 6. Update Portfolio (all in same transaction)
	if err := portfolioStoreTx.UpdatePortfolioWithBuy(userID, symbol, quantity, price); err != nil {
		return nil, err
	}

	// 7. Commit Transaction (all or nothing)
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	// 8. Fetch updated portfolio for response
	userStock, err := s.portfolioStore.GetPortfolioBySymbol(userID, symbol)
	if err != nil {
		// If not found after update, create a response object
		userStock = &data.UserStock{
			UserID:            userID,
			Symbol:            symbol,
			Quantity:          quantity,
			AvgPrice:          price,
			Total:             totalPrice,
			CurrentStockPrice: price,
		}
	} else {
		// Add current stock price to response
		userStock.CurrentStockPrice = price
		userStock.Total = round(userStock.AvgPrice * float64(userStock.Quantity))
	}

	return userStock, nil
}

func (s *InvestmentService) SellStock(userID string, symbol string, quantity int) (*data.UserStock, error) {
	// 1. Get Stock Price from MarketService (Redis-backed)
	stockData, err := s.marketService.GetStock(symbol)
	if err != nil {
		return nil, err
	}
	price := stockData.Price
	totalPrice := round(price * float64(quantity))

	// 2. Start PostgreSQL Transaction (ACID - all operations atomic)
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	userStoreTx := data.NewUserStore(tx)
	tradeStoreTx := data.NewTradesStore(tx)
	portfolioStoreTx := data.NewPortfolioStore(tx)

	// 3. Validate Portfolio (within transaction for consistency)
	existingHolding, err := portfolioStoreTx.GetPortfolioBySymbol(userID, symbol)
	if err != nil {
		if err == data.ErrStockHoldingNotFound {
			return nil, errors.New("stock holding not found")
		}
		return nil, err
	}

	if existingHolding.Quantity < quantity {
		return nil, errors.New("insufficient stock quantity")
	}

	// 4. Update Balance (Add money)
	user, err := userStoreTx.GetUserByID(userID)
	if err != nil {
		return nil, err
	}

	newBalance := user.Balance + totalPrice
	if err := userStoreTx.UpdateBalance(userID, newBalance); err != nil {
		return nil, err
	}

	// 5. Create Trade
	trade := &data.Trade{
		ID:       uuid.New().String(),
		UserID:   userID,
		Symbol:   symbol,
		Action:   "SELL",
		Quantity: quantity,
		Price:    price,
		Date:     time.Now().Format("01/02/2006"),
		Status:   "COMPLETED",
	}

	if err := tradeStoreTx.CreateTrade(trade); err != nil {
		return nil, err
	}

	// 6. Update Portfolio (decrement quantity)
	if err := portfolioStoreTx.UpdatePortfolioWithSell(userID, symbol, quantity); err != nil {
		return nil, err
	}

	// 7. Commit Transaction (all or nothing)
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	// 8. Fetch updated portfolio for response
	userStock, err := s.portfolioStore.GetPortfolioBySymbol(userID, symbol)
	if err != nil {
		if err == data.ErrStockHoldingNotFound {
			// Portfolio was deleted (quantity reached 0), return empty state
			userStock = &data.UserStock{
				UserID:            userID,
				Symbol:            symbol,
				Quantity:          0,
				AvgPrice:          existingHolding.AvgPrice,
				Total:             0,
				CurrentStockPrice: price,
			}
		} else {
			return nil, err
		}
	} else {
		userStock.CurrentStockPrice = price
		userStock.Total = round(userStock.AvgPrice * float64(userStock.Quantity))
	}

	return userStock, nil
}

func (s *InvestmentService) GetUserStocks(userID string) ([]data.UserStock, error) {
	holdings, err := s.portfolioStore.GetPortfolioByUserID(userID)
	if err != nil {
		return nil, err
	}

	// Enrich with current stock prices
	for i := range holdings {
		stockData, err := s.marketService.GetStock(holdings[i].Symbol)
		if err == nil && stockData != nil {
			holdings[i].CurrentStockPrice = stockData.Price
			holdings[i].Total = round(holdings[i].AvgPrice * float64(holdings[i].Quantity))
		}
	}

	return holdings, nil
}
