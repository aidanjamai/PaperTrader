package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/shopspring/decimal"

	"papertrader/internal/data"
	"papertrader/internal/util"
)

// MarketPricer is the subset of MarketService used by InvestmentService.
// Defined here to allow testing without a live MarketStack API.
type MarketPricer interface {
	GetStock(ctx context.Context, symbol string) (*StockData, error)
	GetBatchHistoricalData(ctx context.Context, symbols []string) (map[string]*HistoricalData, error)
}

type InvestmentService struct {
	db             *sql.DB
	marketService  MarketPricer
	portfolioStore *data.PortfolioStore
	tradesStore    *data.TradesStore
}

func NewInvestmentService(db *sql.DB, marketService MarketPricer, portfolioStore *data.PortfolioStore, tradesStore *data.TradesStore) *InvestmentService {
	return &InvestmentService{
		db:             db,
		marketService:  marketService,
		portfolioStore: portfolioStore,
		tradesStore:    tradesStore,
	}
}

func (s *InvestmentService) BuyStock(ctx context.Context, userID string, symbol string, quantity int, idempotencyKey string) (*data.UserStock, error) {
	// Validate quantity (defense in depth)
	if err := util.ValidateQuantity(quantity); err != nil {
		return nil, err
	}

	// Idempotency pre-check: if key provided and trade already exists, return replay.
	if idempotencyKey != "" {
		existing, err := s.tradesStore.GetTradeByIdempotencyKey(ctx, userID, idempotencyKey)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			return s.buildBuyReplay(ctx, userID, existing)
		}
	}

	// 1. Get Stock Price from MarketService (Redis-backed)
	stockData, err := s.marketService.GetStock(ctx, symbol)
	if err != nil {
		return nil, err
	}
	price := stockData.Price
	totalPrice := price.Mul(decimal.NewFromInt(int64(quantity)))

	// 2. Start PostgreSQL Transaction (ACID - all operations atomic)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	userStoreTx := data.NewUserStore(tx)
	tradeStoreTx := data.NewTradesStore(tx)
	portfolioStoreTx := data.NewPortfolioStore(tx)

	// 3. Get User Balance with row lock and Validate.
	// FOR UPDATE prevents two concurrent buys from both reading the same balance
	// and both passing the funds check; the second buy now blocks until the
	// first's tx commits or rolls back.
	balance, err := userStoreTx.GetBalanceForUpdate(ctx, userID)
	if err != nil {
		return nil, err
	}

	if balance.LessThan(totalPrice) {
		return nil, &InsufficientFundsError{}
	}

	// 4. Deduct Balance
	newBalance := balance.Sub(totalPrice)
	if err := userStoreTx.UpdateBalance(ctx, userID, newBalance); err != nil {
		return nil, err
	}

	// 5. Create Trade — executed_at is filled by the DB default.
	trade := &data.Trade{
		ID:             uuid.New().String(),
		UserID:         userID,
		Symbol:         symbol,
		Action:         "BUY",
		Quantity:       quantity,
		Price:          price,
		Status:         "COMPLETED",
		IdempotencyKey: idempotencyKey,
	}

	if err := tradeStoreTx.CreateTrade(ctx, trade); err != nil {
		// Unique violation on idempotency key — concurrent retry won the race.
		// Roll back and return the existing trade result.
		var pqErr *pq.Error
		if idempotencyKey != "" && errors.As(err, &pqErr) && pqErr.Code == "23505" {
			tx.Rollback()
			existing, fetchErr := s.tradesStore.GetTradeByIdempotencyKey(ctx, userID, idempotencyKey)
			if fetchErr != nil {
				return nil, fetchErr
			}
			if existing != nil {
				return s.buildBuyReplay(ctx, userID, existing)
			}
			// Re-fetch came back empty: the conflicting row was rolled back by
			// its own transaction, leaving the unique violation we observed
			// orphaned. Surface a wrapped error rather than the raw *pq.Error
			// so callers see a stable string and don't depend on driver internals.
			return nil, fmt.Errorf("idempotency conflict but no prior trade found: %w", err)
		}
		return nil, err
	}

	// 6. Update Portfolio (all in same transaction)
	if err := portfolioStoreTx.UpdatePortfolioWithBuy(ctx, userID, symbol, quantity, price); err != nil {
		return nil, err
	}

	// 7. Commit Transaction (all or nothing)
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	slog.Info("trade executed",
		"action", "BUY",
		"user_id", userID,
		"symbol", symbol,
		"quantity", quantity,
		"price", price,
		"new_balance", newBalance,
	)

	// 8. Fetch updated portfolio for response
	userStock, err := s.portfolioStore.GetPortfolioBySymbol(ctx, userID, symbol)
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
		userStock.Total = userStock.AvgPrice.Mul(decimal.NewFromInt(int64(userStock.Quantity)))
	}

	return userStock, nil
}

// buildBuyReplay fetches current portfolio state for a previously-recorded BUY
// and returns it as a UserStock. Used for idempotency replays so no new tx is opened.
func (s *InvestmentService) buildBuyReplay(ctx context.Context, userID string, trade *data.Trade) (*data.UserStock, error) {
	userStock, err := s.portfolioStore.GetPortfolioBySymbol(ctx, userID, trade.Symbol)
	if err != nil {
		// Only fall through for a clean "not found" — the holding may have been
		// subsequently sold. Any other error (e.g. transient DB failure) must propagate.
		if err != data.ErrStockHoldingNotFound {
			return nil, err
		}
		userStock = &data.UserStock{
			UserID:            userID,
			Symbol:            trade.Symbol,
			Quantity:          trade.Quantity,
			AvgPrice:          trade.Price,
			Total:             trade.Price.Mul(decimal.NewFromInt(int64(trade.Quantity))),
			CurrentStockPrice: trade.Price,
		}
	} else {
		userStock.CurrentStockPrice = trade.Price
		userStock.Total = userStock.AvgPrice.Mul(decimal.NewFromInt(int64(userStock.Quantity)))
	}
	return userStock, nil
}

func (s *InvestmentService) SellStock(ctx context.Context, userID string, symbol string, quantity int, idempotencyKey string) (*data.UserStock, error) {
	// Validate quantity (defense in depth)
	if err := util.ValidateQuantity(quantity); err != nil {
		return nil, err
	}

	// Idempotency pre-check: if key provided and trade already exists, return replay.
	if idempotencyKey != "" {
		existing, err := s.tradesStore.GetTradeByIdempotencyKey(ctx, userID, idempotencyKey)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			return s.buildSellReplay(ctx, userID, existing)
		}
	}

	// 1. Get Stock Price from MarketService (Redis-backed)
	stockData, err := s.marketService.GetStock(ctx, symbol)
	if err != nil {
		return nil, err
	}
	price := stockData.Price
	totalPrice := price.Mul(decimal.NewFromInt(int64(quantity)))

	// 2. Start PostgreSQL Transaction (ACID - all operations atomic)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	userStoreTx := data.NewUserStore(tx)
	tradeStoreTx := data.NewTradesStore(tx)
	portfolioStoreTx := data.NewPortfolioStore(tx)

	// 3. Validate Portfolio with row lock — prevents two concurrent sells of
	// the same holding from both passing the quantity check and overselling.
	existingHolding, err := portfolioStoreTx.GetPortfolioBySymbolForUpdate(ctx, userID, symbol)
	if err != nil {
		if err == data.ErrStockHoldingNotFound {
			return nil, &StockHoldingNotFoundError{}
		}
		return nil, err
	}

	if existingHolding.Quantity < quantity {
		return nil, &InsufficientStockError{}
	}

	// 4. Read+lock balance, then credit the proceeds. The lock matters even on
	// the credit side: without it a concurrent buy on the same user could
	// commit between this read and our UPDATE, and last-writer-wins would
	// silently drop the buy's deduction.
	balance, err := userStoreTx.GetBalanceForUpdate(ctx, userID)
	if err != nil {
		return nil, err
	}

	newBalance := balance.Add(totalPrice)
	if err := userStoreTx.UpdateBalance(ctx, userID, newBalance); err != nil {
		return nil, err
	}

	// 5. Create Trade — executed_at is filled by the DB default.
	trade := &data.Trade{
		ID:             uuid.New().String(),
		UserID:         userID,
		Symbol:         symbol,
		Action:         "SELL",
		Quantity:       quantity,
		Price:          price,
		Status:         "COMPLETED",
		IdempotencyKey: idempotencyKey,
	}

	if err := tradeStoreTx.CreateTrade(ctx, trade); err != nil {
		// Unique violation on idempotency key — concurrent retry won the race.
		var pqErr *pq.Error
		if idempotencyKey != "" && errors.As(err, &pqErr) && pqErr.Code == "23505" {
			tx.Rollback()
			existing, fetchErr := s.tradesStore.GetTradeByIdempotencyKey(ctx, userID, idempotencyKey)
			if fetchErr != nil {
				return nil, fetchErr
			}
			if existing != nil {
				return s.buildSellReplay(ctx, userID, existing)
			}
			// See BuyStock for rationale on the wrapped error.
			return nil, fmt.Errorf("idempotency conflict but no prior trade found: %w", err)
		}
		return nil, err
	}

	// 6. Update Portfolio (decrement quantity). Pass the locked quantity we
	// already read above; the store no longer re-reads it.
	if err := portfolioStoreTx.UpdatePortfolioWithSell(ctx, userID, symbol, existingHolding.Quantity, quantity); err != nil {
		return nil, err
	}

	// 7. Commit Transaction (all or nothing)
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	slog.Info("trade executed",
		"action", "SELL",
		"user_id", userID,
		"symbol", symbol,
		"quantity", quantity,
		"price", price,
		"new_balance", newBalance,
	)

	// 8. Fetch updated portfolio for response
	userStock, err := s.portfolioStore.GetPortfolioBySymbol(ctx, userID, symbol)
	if err != nil {
		if err == data.ErrStockHoldingNotFound {
			// Portfolio was deleted (quantity reached 0), return empty state
			userStock = &data.UserStock{
				UserID:            userID,
				Symbol:            symbol,
				Quantity:          0,
				AvgPrice:          existingHolding.AvgPrice,
				Total:             decimal.Zero,
				CurrentStockPrice: price,
			}
		} else {
			return nil, err
		}
	} else {
		userStock.CurrentStockPrice = price
		userStock.Total = userStock.AvgPrice.Mul(decimal.NewFromInt(int64(userStock.Quantity)))
	}

	return userStock, nil
}

// buildSellReplay returns current portfolio state for a previously-recorded SELL.
func (s *InvestmentService) buildSellReplay(ctx context.Context, userID string, trade *data.Trade) (*data.UserStock, error) {
	userStock, err := s.portfolioStore.GetPortfolioBySymbol(ctx, userID, trade.Symbol)
	if err != nil {
		// Only fall through for a clean "not found" — the holding may have been
		// fully sold. Any other error (e.g. transient DB failure) must propagate.
		if err != data.ErrStockHoldingNotFound {
			return nil, err
		}
		userStock = &data.UserStock{
			UserID:            userID,
			Symbol:            trade.Symbol,
			Quantity:          0,
			AvgPrice:          trade.Price,
			Total:             decimal.Zero,
			CurrentStockPrice: trade.Price,
		}
	} else {
		userStock.CurrentStockPrice = trade.Price
		userStock.Total = userStock.AvgPrice.Mul(decimal.NewFromInt(int64(userStock.Quantity)))
	}
	return userStock, nil
}

// GetUserStocks returns all portfolio holdings enriched with current prices.
// It uses a single batch call to GetBatchHistoricalData (24h cache) instead of
// per-symbol GetStock calls to stay within MarketStack's free-tier limits.
// If the batch fetch fails the holdings are still returned; CurrentStockPrice
// will be 0 for symbols whose prices could not be retrieved.
func (s *InvestmentService) GetUserStocks(ctx context.Context, userID string) ([]data.UserStock, error) {
	holdings, err := s.portfolioStore.GetPortfolioByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	if len(holdings) > 0 {
		symbols := make([]string, len(holdings))
		for i, h := range holdings {
			symbols[i] = h.Symbol
		}

		// Batch price lookup — partial failures are logged but don't block the response.
		priceData, priceErr := s.marketService.GetBatchHistoricalData(ctx, symbols)
		if priceErr != nil {
			slog.Warn("batch price fetch failed; prices may be stale",
				"user_id", userID,
				"err", priceErr,
				"component", "investment",
			)
		}
		for i := range holdings {
			holdings[i].Total = holdings[i].AvgPrice.Mul(decimal.NewFromInt(int64(holdings[i].Quantity)))
			if priceData != nil {
				if hist, ok := priceData[holdings[i].Symbol]; ok && hist != nil {
					holdings[i].CurrentStockPrice = hist.Price
				}
			}
		}
	}

	return holdings, nil
}

// GetUserTrades returns a page of trades for the user along with the total
// count matching the same filter (used by the UI for pagination state).
// Both queries run on the non-transactional trades store; the trades log is
// append-only so a missed-row anomaly between the two queries is not a concern.
func (s *InvestmentService) GetUserTrades(ctx context.Context, userID string, opts data.TradeQueryOpts) ([]data.Trade, int, error) {
	trades, err := s.tradesStore.GetTradesByUserID(ctx, userID, opts)
	if err != nil {
		return nil, 0, err
	}
	total, err := s.tradesStore.CountTradesByUserID(ctx, userID, opts)
	if err != nil {
		return nil, 0, err
	}
	return trades, total, nil
}
