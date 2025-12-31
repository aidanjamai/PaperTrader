import React, { useState, useEffect, useCallback, useMemo, memo } from 'react';
import { apiRequest } from '../../services/api';
import { StockResponse, HistoricalDataResponse, User, BuyStockRequest, SellStockRequest, UserStock } from '../../types';
import { useAuth } from '../../hooks/useAuth';

interface StockListing {
  symbol: string;
  name: string;
  type: 'stock' | 'index';
}

interface StockData extends StockListing {
  price: number;
  change: number;
  changePercentage: number;
  date: string;
  loading?: boolean;
  error?: string;
}

// Popular stocks and index funds
const POPULAR_STOCKS: StockListing[] = [
  { symbol: 'AAPL', name: 'Apple Inc.', type: 'stock' },
  { symbol: 'GOOGL', name: 'Alphabet Inc.', type: 'stock' },
  { symbol: 'MSFT', name: 'Microsoft Corporation', type: 'stock' },
  { symbol: 'AMZN', name: 'Amazon.com Inc.', type: 'stock' },
  { symbol: 'TSLA', name: 'Tesla Inc.', type: 'stock' },
  { symbol: 'META', name: 'Meta Platforms Inc.', type: 'stock' },
  { symbol: 'NVDA', name: 'NVIDIA Corporation', type: 'stock' },
  { symbol: 'JPM', name: 'JPMorgan Chase & Co.', type: 'stock' },
  { symbol: 'V', name: 'Visa Inc.', type: 'stock' },
  { symbol: 'JNJ', name: 'Johnson & Johnson', type: 'stock' },
];

const POPULAR_INDEXES: StockListing[] = [
  { symbol: 'SPY', name: 'SPDR S&P 500 ETF', type: 'index' },
  { symbol: 'QQQ', name: 'Invesco QQQ Trust', type: 'index' },
  { symbol: 'DIA', name: 'SPDR Dow Jones Industrial Average', type: 'index' },
  { symbol: 'IWM', name: 'iShares Russell 2000 ETF', type: 'index' },
  { symbol: 'VTI', name: 'Vanguard Total Stock Market ETF', type: 'index' },
];

interface TradeModalProps {
  stock: StockData | null;
  user: User | null;
  isOpen: boolean;
  onClose: () => void;
  onTradeSuccess: () => void;
}

const TradeModal: React.FC<TradeModalProps> = ({ stock, user, isOpen, onClose, onTradeSuccess }) => {
  const [tradeType, setTradeType] = useState<'buy' | 'sell'>('buy');
  const [quantity, setQuantity] = useState<string>('');
  const [loading, setLoading] = useState<boolean>(false);
  const [error, setError] = useState<string>('');
  const [message, setMessage] = useState<string>('');

  useEffect(() => {
    if (!isOpen) {
      // Reset form when modal closes
      setQuantity('');
      setError('');
      setMessage('');
      setTradeType('buy');
    }
  }, [isOpen]);

  const handleSubmit = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    if (!stock || !user) return;

    setError('');
    setMessage('');
    setLoading(true);

    const quantityNum = parseInt(quantity, 10);
    if (isNaN(quantityNum) || quantityNum <= 0) {
      setError('Quantity must be a positive whole number');
      setLoading(false);
      return;
    }

    try {
      const endpoint = tradeType === 'buy' ? '/investments/buy' : '/investments/sell';
      const requestBody: BuyStockRequest | SellStockRequest = {
        symbol: stock.symbol,
        quantity: quantityNum
      };
      
      const response = await apiRequest<UserStock>(endpoint, {
        method: 'POST',
        body: JSON.stringify(requestBody)
      });

      if (response.ok) {
        await response.json();
        setMessage(`${tradeType === 'buy' ? 'Bought' : 'Sold'} ${quantityNum} shares of ${stock.symbol} successfully!`);
        setQuantity('');
        setTimeout(() => {
          onTradeSuccess();
          onClose();
        }, 1500);
      } else {
        const errorData = await response.text();
        try {
          const jsonError = JSON.parse(errorData) as { message?: string };
          setError(jsonError.message || errorData);
        } catch {
          setError(errorData || `${tradeType === 'buy' ? 'Buy' : 'Sell'} failed`);
        }
      }
    } catch (error) {
      setError('Network error. Please try again.');
    } finally {
      setLoading(false);
    }
  };

  if (!isOpen || !stock || !user) return null;

  const quantityNum = parseInt(quantity, 10) || 0;
  const totalCost = quantityNum * stock.price;
  const remainingBalance = tradeType === 'buy' 
    ? user.balance - totalCost 
    : user.balance + totalCost;

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal-content" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <h2>{stock.name} ({stock.symbol})</h2>
          <button className="modal-close" onClick={onClose}>&times;</button>
        </div>

        <div className="modal-body">
          {/* Current Price Display */}
          <div style={{ 
            background: '#f8f9fa', 
            padding: '16px', 
            borderRadius: '8px', 
            marginBottom: '20px',
            textAlign: 'center'
          }}>
            <div style={{ color: '#666', fontSize: '14px', marginBottom: '4px' }}>Current Price</div>
            <div style={{ fontSize: '32px', fontWeight: 'bold', color: '#333' }}>
              ${stock.price.toFixed(2)}
            </div>
            <div style={{ 
              fontSize: '14px', 
              color: stock.change >= 0 ? '#10b981' : '#ef4444',
              marginTop: '8px'
            }}>
              {stock.change >= 0 ? '+' : ''}{stock.change.toFixed(2)} ({stock.changePercentage >= 0 ? '+' : ''}{stock.changePercentage.toFixed(2)}%)
            </div>
          </div>

          {/* Trade Type Toggle */}
          <div className="form-group">
            <label>Trade Type</label>
            <div style={{ display: 'flex', gap: '10px', marginTop: '8px' }}>
              <button
                type="button"
                className={`btn ${tradeType === 'buy' ? 'btn-primary' : 'btn-secondary'}`}
                onClick={() => setTradeType('buy')}
                style={{ flex: 1 }}
              >
                Buy
              </button>
              <button
                type="button"
                className={`btn ${tradeType === 'sell' ? 'btn-primary' : 'btn-secondary'}`}
                onClick={() => setTradeType('sell')}
                style={{ flex: 1 }}
              >
                Sell
              </button>
            </div>
          </div>

          {error && (
            <div className="alert alert-error">
              {error}
            </div>
          )}

          {message && (
            <div className="alert alert-success">
              {message}
            </div>
          )}

          <form onSubmit={handleSubmit}>
            <div className="form-group">
              <label htmlFor="quantity">Quantity</label>
              <input
                type="number"
                id="quantity"
                className="form-control"
                value={quantity}
                onChange={(e) => setQuantity(e.target.value)}
                placeholder="Enter number of shares"
                required
                disabled={loading}
                min="1"
                step="1"
              />
              <small style={{ color: '#666', fontSize: '12px' }}>
                Must be a whole positive number
              </small>
            </div>

            {/* Live Calculations */}
            {quantityNum > 0 && (
              <div style={{ 
                background: '#f8f9fa', 
                padding: '16px', 
                borderRadius: '8px', 
                marginBottom: '20px' 
              }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '8px' }}>
                  <span style={{ color: '#666' }}>Shares:</span>
                  <strong>{quantityNum}</strong>
                </div>
                <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '8px' }}>
                  <span style={{ color: '#666' }}>Price per share:</span>
                  <strong>${stock.price.toFixed(2)}</strong>
                </div>
                <div style={{ 
                  display: 'flex', 
                  justifyContent: 'space-between', 
                  marginBottom: '12px',
                  paddingTop: '12px',
                  borderTop: '1px solid #dee2e6'
                }}>
                  <span style={{ color: '#666', fontWeight: '600' }}>
                    {tradeType === 'buy' ? 'Total Cost:' : 'Total Proceeds:'}
                  </span>
                  <strong style={{ fontSize: '18px', color: '#333' }}>
                    ${totalCost.toFixed(2)}
                  </strong>
                </div>
                <div style={{ 
                  display: 'flex', 
                  justifyContent: 'space-between',
                  paddingTop: '12px',
                  borderTop: '1px solid #dee2e6'
                }}>
                  <span style={{ color: '#666' }}>Current Balance:</span>
                  <strong>${user.balance.toFixed(2)}</strong>
                </div>
                <div style={{ 
                  display: 'flex', 
                  justifyContent: 'space-between',
                  marginTop: '8px'
                }}>
                  <span style={{ color: '#666', fontWeight: '600' }}>Remaining Balance:</span>
                  <strong style={{ 
                    fontSize: '18px', 
                    color: remainingBalance < 0 ? '#ef4444' : '#10b981' 
                  }}>
                    ${remainingBalance.toFixed(2)}
                  </strong>
                </div>
                {remainingBalance < 0 && tradeType === 'buy' && (
                  <div style={{ 
                    color: '#ef4444', 
                    fontSize: '12px', 
                    marginTop: '8px',
                    textAlign: 'center'
                  }}>
                    Insufficient funds
                  </div>
                )}
              </div>
            )}

            <button
              type="submit"
              className={`btn ${tradeType === 'buy' ? 'btn-success' : 'btn-warning'}`}
              style={{ width: '100%' }}
              disabled={loading || (quantityNum > 0 && remainingBalance < 0 && tradeType === 'buy')}
            >
              {loading ? `${tradeType === 'buy' ? 'Buying' : 'Selling'}...` : `${tradeType === 'buy' ? 'Buy' : 'Sell'} Stock`}
            </button>
          </form>
        </div>
      </div>
    </div>
  );
};

// localStorage keys
const STOCKS_CACHE_KEY = 'papertrader_markets_stocks';
const INDEXES_CACHE_KEY = 'papertrader_markets_indexes';
const CACHE_TIMESTAMP_KEY = 'papertrader_markets_cache_timestamp';

// Cache expires at end of trading day (4 PM EST) or next day
// Stock data doesn't change until the next trading day
function isCacheValid(timestamp: number): boolean {
  if (!timestamp || timestamp <= 0) return false;
  
  const now = new Date();
  const cacheDate = new Date(timestamp);
  
  // Check if cache is from today
  const isToday = cacheDate.toDateString() === now.toDateString();
  
  if (!isToday) {
    // Cache is from a different day, check if it's still valid
    // Cache expires at 4 PM EST on the day it was created
    // For simplicity, we'll expire cache after 24 hours (next trading day)
    const hoursSinceCache = (now.getTime() - timestamp) / (1000 * 60 * 60);
    return hoursSinceCache < 24;
  }
  
  // Cache is from today, check if it's before 4 PM EST
  // For simplicity, expire at midnight local time (next day)
  // This ensures fresh data each day
  const tomorrow = new Date(now);
  tomorrow.setHours(0, 0, 0, 0);
  tomorrow.setDate(tomorrow.getDate() + 1);
  
  return now.getTime() < tomorrow.getTime();
}

interface CachedData {
  stocks: StockData[];
  indexes: StockData[];
  timestamp: number;
}

const Markets: React.FC = () => {
  const { user, isAuthenticated } = useAuth();
  const [searchQuery, setSearchQuery] = useState<string>('');
  const [stocks, setStocks] = useState<StockData[]>([]);
  const [indexes, setIndexes] = useState<StockData[]>([]);
  const [selectedStock, setSelectedStock] = useState<StockData | null>(null);
  const [isModalOpen, setIsModalOpen] = useState<boolean>(false);
  const [loading, setLoading] = useState<boolean>(true);
  const [searchResult, setSearchResult] = useState<StockData | null>(null);
  const [searchLoading, setSearchLoading] = useState<boolean>(false);

  // Load cached data from localStorage
  const loadCachedData = useCallback((): CachedData | null => {
    try {
      const cachedStocks = localStorage.getItem(STOCKS_CACHE_KEY);
      const cachedIndexes = localStorage.getItem(INDEXES_CACHE_KEY);
      const cachedTimestamp = localStorage.getItem(CACHE_TIMESTAMP_KEY);

      if (!cachedStocks || !cachedIndexes || !cachedTimestamp) {
        return null;
      }

      const timestamp = parseInt(cachedTimestamp, 10);

      // Check if cache is still valid (until end of trading day)
      if (!isCacheValid(timestamp)) {
        console.log('[Markets] Cache expired (past end of trading day), will fetch fresh data');
        return null;
      }

      const stocksData = JSON.parse(cachedStocks) as StockData[];
      const indexesData = JSON.parse(cachedIndexes) as StockData[];

      // Validate cached data structure
      if (!Array.isArray(stocksData) || !Array.isArray(indexesData)) {
        console.warn('[Markets] Cached data has invalid structure, will fetch fresh data');
        return null;
      }

      // Check if cached data is actually populated
      if (stocksData.length === 0 && indexesData.length === 0) {
        console.warn('[Markets] Cached data is empty, will fetch fresh data');
        return null;
      }

      console.log(`[Markets] Loaded cached data from localStorage: ${stocksData.length} stocks, ${indexesData.length} indexes`);
      return {
        stocks: stocksData,
        indexes: indexesData,
        timestamp,
      };
    } catch (error) {
      console.error('[Markets] Error loading cached data:', error);
      return null;
    }
  }, []);

  // Save data to localStorage
  const saveCachedData = useCallback((stocksData: StockData[], indexesData: StockData[]) => {
    try {
      localStorage.setItem(STOCKS_CACHE_KEY, JSON.stringify(stocksData));
      localStorage.setItem(INDEXES_CACHE_KEY, JSON.stringify(indexesData));
      localStorage.setItem(CACHE_TIMESTAMP_KEY, Date.now().toString());
      console.log('[Markets] Saved data to localStorage cache');
    } catch (error) {
      console.error('[Markets] Error saving cached data:', error);
    }
  }, []);

  // Fetch batch stock data using the new batch endpoint (much more efficient)
  const fetchBatchStockData = useCallback(async (symbols: string[]): Promise<Map<string, StockData>> => {
    const result = new Map<string, StockData>();
    
    if (symbols.length === 0) {
      return result;
    }

    try {
      // URL encode symbols to handle special characters safely
      const encodedSymbols = symbols.map(s => encodeURIComponent(s)).join(',');
      // Use batch endpoint to fetch all symbols in one request
      const response = await apiRequest<{ [symbol: string]: HistoricalDataResponse }>(
        `/market/stock/historical/daily/batch?symbols=${encodedSymbols}`
      );

      if (!response.ok) {
        console.error(`[Markets] Batch fetch failed: ${response.status}`);
        return result;
      }

      const responseData = await response.json() as { success: boolean; message: string; data: { [symbol: string]: HistoricalDataResponse } };
      const batchData = responseData.data || responseData as unknown as { [symbol: string]: HistoricalDataResponse };

      // Convert batch response to StockData map
      for (const symbol of symbols) {
        const historicalData = batchData[symbol];
        if (!historicalData || historicalData.price === undefined) {
          console.warn(`[Markets] No data for ${symbol} in batch response`);
          continue;
        }

        const listing = [...POPULAR_STOCKS, ...POPULAR_INDEXES].find(s => s.symbol === symbol);
        result.set(symbol, {
          symbol: historicalData.symbol,
          name: listing?.name || symbol,
          type: listing?.type || 'stock',
          price: historicalData.price,
          change: historicalData.change || 0,
          changePercentage: historicalData.change_percentage || 0,
          date: historicalData.date,
        });
      }

      console.log(`[Markets] Batch fetch completed: ${result.size}/${symbols.length} symbols retrieved`);
    } catch (error) {
      console.error('[Markets] Error in batch fetch:', error);
    }

    return result;
  }, []);

  // Fetch stock data for a single symbol (fallback for search)
  const fetchStockData = useCallback(async (symbol: string): Promise<StockData | null> => {
    try {
      // Fetch historical data which includes current price, change, and change percentage
      const historicalResponse = await apiRequest<HistoricalDataResponse>(`/market/stock/historical/daily?symbol=${symbol}`);
      
      if (!historicalResponse.ok) {
        // If historical data fails (e.g., insufficient data), fall back to price endpoint
        console.warn(`Historical data unavailable for ${symbol} (${historicalResponse.status}), trying price endpoint...`);
        
        try {
          const priceResponse = await apiRequest<StockResponse>(`/market/stock?symbol=${symbol}`);
          if (!priceResponse.ok) {
            console.warn(`Failed to fetch price for ${symbol}: ${priceResponse.status}`);
            return null;
          }

          const priceDataResponse = await priceResponse.json() as { success: boolean; message: string; data: StockResponse };
          const priceData = priceDataResponse.data || priceDataResponse as unknown as StockResponse;

          const listing = [...POPULAR_STOCKS, ...POPULAR_INDEXES].find(s => s.symbol === symbol);
          
          // Return with 0 change if we only have price data
          return {
            symbol: priceData.symbol,
            name: listing?.name || symbol,
            type: listing?.type || 'stock',
            price: priceData.price,
            change: 0,
            changePercentage: 0,
            date: priceData.date,
          };
        } catch (fallbackError) {
          console.error(`Fallback price fetch failed for ${symbol}:`, fallbackError);
          return null;
        }
      }

      // Parse historical data response
      let historicalData: HistoricalDataResponse;
      try {
        const historicalDataResponse = await historicalResponse.json() as { success: boolean; message: string; data: HistoricalDataResponse };
        historicalData = historicalDataResponse.data || historicalDataResponse as unknown as HistoricalDataResponse;
      } catch (parseError) {
        console.error(`Failed to parse historical response for ${symbol}:`, parseError);
        return null;
      }

      if (!historicalData || historicalData.price === undefined) {
        console.warn(`Invalid historical data for ${symbol}`);
        return null;
      }

      const listing = [...POPULAR_STOCKS, ...POPULAR_INDEXES].find(s => s.symbol === symbol);
      
      return {
        symbol: historicalData.symbol,
        name: listing?.name || symbol,
        type: listing?.type || 'stock',
        price: historicalData.price, // Current price from historical data
        change: historicalData.change || 0,
        changePercentage: historicalData.change_percentage || 0,
        date: historicalData.date,
      };
    } catch (error) {
      console.error(`Error fetching data for ${symbol}:`, error);
      return null;
    }
  }, []);

  // Extract shared logic for processing batch data
  const processBatchData = useCallback((batchData: Map<string, StockData>) => {
    const stockResults: StockData[] = [];
    const indexResults: StockData[] = [];

    for (const stock of POPULAR_STOCKS) {
      const data = batchData.get(stock.symbol);
      if (data && data.price !== undefined) {
        stockResults.push({ ...stock, ...data });
      }
    }

    for (const index of POPULAR_INDEXES) {
      const data = batchData.get(index.symbol);
      if (data && data.price !== undefined) {
        indexResults.push({ ...index, ...data });
      }
    }

    return { stockResults, indexResults };
  }, []);

  // Shared function to fetch and process stock data
  const fetchAndProcessStocks = useCallback(async (forceRefresh: boolean = false) => {
    // Check cache first (unless forcing refresh)
    if (!forceRefresh) {
      const cached = loadCachedData();
      if (cached) {
        // Validate cached data before using it
        const hasStocks = cached.stocks && Array.isArray(cached.stocks) && cached.stocks.length > 0;
        const hasIndexes = cached.indexes && Array.isArray(cached.indexes) && cached.indexes.length > 0;
        
        if (hasStocks || hasIndexes) {
          setStocks(cached.stocks || []);
          setIndexes(cached.indexes || []);
          setLoading(false);
          console.log(`[Markets] Using cached data: ${cached.stocks?.length || 0} stocks, ${cached.indexes?.length || 0} indexes`);
          return;
        } else {
          // Clear invalid/empty cache and fetch fresh data
          console.warn('[Markets] Cached data is empty or invalid, clearing cache and fetching fresh data');
          localStorage.removeItem(STOCKS_CACHE_KEY);
          localStorage.removeItem(INDEXES_CACHE_KEY);
          localStorage.removeItem(CACHE_TIMESTAMP_KEY);
        }
      }
    }

    console.log('[Markets] Fetching fresh data from API using batch endpoint...');
    
    // Collect all symbols
    const allSymbols = [
      ...POPULAR_STOCKS.map(s => s.symbol),
      ...POPULAR_INDEXES.map(s => s.symbol)
    ];

    // Fetch all symbols in one batch request
    const batchData = await fetchBatchStockData(allSymbols);
    const { stockResults, indexResults } = processBatchData(batchData);

    setStocks(stockResults);
    setIndexes(indexResults);
    
    // Save to cache
    if (stockResults.length > 0 || indexResults.length > 0) {
      saveCachedData(stockResults, indexResults);
    }
    
    setLoading(false);
  }, [fetchBatchStockData, loadCachedData, saveCachedData, processBatchData]);

  // Load all popular stocks and indexes using batch endpoint (single API call)
  useEffect(() => {
    let isMounted = true;

    const loadStocks = async () => {
      setLoading(true);
      await fetchAndProcessStocks(false);
      if (!isMounted) return;
    };

    loadStocks().catch((error) => {
      console.error('[Markets] Error loading stocks:', error);
      if (isMounted) {
        setLoading(false);
      }
    });

    return () => {
      isMounted = false;
    };
  }, [fetchAndProcessStocks]);

  // Expose refresh function (can be called manually)
  const handleRefresh = useCallback(async () => {
    // Clear cache and reload
    localStorage.removeItem(STOCKS_CACHE_KEY);
    localStorage.removeItem(INDEXES_CACHE_KEY);
    localStorage.removeItem(CACHE_TIMESTAMP_KEY);
    
    setLoading(true);
    try {
      await fetchAndProcessStocks(true);
    } catch (error) {
      console.error('[Markets] Error refreshing stocks:', error);
      setLoading(false);
    }
  }, [fetchAndProcessStocks]);

  // Handle search - check cache first, then API
  const handleSearch = useCallback(async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    const trimmedQuery = searchQuery.trim();
    if (!trimmedQuery) return;

    setSearchLoading(true);
    setSearchResult(null);
    
    // Sanitize and validate symbol input
    const symbol = trimmedQuery.toUpperCase().replace(/[^A-Z.]/g, '');
    if (!symbol) {
      setSearchResult({
        symbol: trimmedQuery,
        name: trimmedQuery,
        type: 'stock',
        price: 0,
        change: 0,
        changePercentage: 0,
        date: '',
        error: 'Invalid symbol format'
      });
      setSearchLoading(false);
      return;
    }
    
    // Check if symbol is already in cached stocks/indexes
    const allCachedStocks = [...stocks, ...indexes];
    const cachedStock = allCachedStocks.find(s => s.symbol === symbol);
    
    if (cachedStock) {
      setSearchResult(cachedStock);
      setSearchLoading(false);
      console.log(`[Markets] Search result for ${symbol} found in cache`);
      return;
    }
    
    // Not in cache, fetch from API
    try {
      const data = await fetchStockData(symbol);
      
      if (data) {
        setSearchResult(data);
      } else {
        setSearchResult({
          symbol,
          name: symbol,
          type: 'stock',
          price: 0,
          change: 0,
          changePercentage: 0,
          date: '',
          error: 'Stock not found'
        });
      }
    } catch (error) {
      console.error(`[Markets] Error searching for ${symbol}:`, error);
      setSearchResult({
        symbol,
        name: symbol,
        type: 'stock',
        price: 0,
        change: 0,
        changePercentage: 0,
        date: '',
        error: 'Error fetching stock data. Please try again.'
      });
    } finally {
      setSearchLoading(false);
    }
  }, [searchQuery, stocks, indexes, fetchStockData]);

  const handleStockClick = useCallback((stock: StockData) => {
    if (!isAuthenticated || !user) {
      // Better UX: show a more user-friendly message
      setSearchResult({
        ...stock,
        error: 'Please log in to trade stocks'
      });
      return;
    }
    
    // Check if email is verified
    if (!user.email_verified) {
      setSearchResult({
        ...stock,
        error: 'Please verify your email address to trade stocks. Check the banner at the top of the page for instructions.'
      });
      return;
    }
    
    setSelectedStock(stock);
    setIsModalOpen(true);
  }, [isAuthenticated, user]);

  const handleTradeSuccess = useCallback(() => {
    // After successful trade, just close modal - stock prices don't change from trading
    // The portfolio will be updated by the parent component
    setIsModalOpen(false);
  }, []);

  // Memoize change color function
  const getChangeColor = useCallback((change: number) => {
    if (change > 0) return '#10b981';
    if (change < 0) return '#ef4444';
    return '#666';
  }, []);

  // Memoize stock row component to prevent unnecessary re-renders
  // Using a separate component to allow useState
  const StockRowComponent: React.FC<{ stock: StockData; onStockClick: (stock: StockData) => void }> = ({ stock, onStockClick }) => {
    const [isHovered, setIsHovered] = useState(false);

    return (
      <tr 
        role="button"
        tabIndex={0}
        aria-label={`${stock.symbol} - ${stock.name} - Price: $${stock.price.toFixed(2)}`}
        style={{ 
          cursor: 'pointer',
          transition: 'background-color 0.2s ease',
          backgroundColor: isHovered ? '#f8f9fa' : 'transparent'
        }}
        onClick={() => onStockClick(stock)}
        onMouseEnter={() => setIsHovered(true)}
        onMouseLeave={() => setIsHovered(false)}
        onKeyDown={(e) => {
          if (e.key === 'Enter' || e.key === ' ') {
            e.preventDefault();
            onStockClick(stock);
          }
        }}
      >
      <td style={{ padding: '16px' }}>
        <div>
          <strong style={{ fontSize: '16px', color: '#333' }}>{stock.symbol}</strong>
          <div style={{ fontSize: '12px', color: '#666', marginTop: '4px' }}>{stock.name}</div>
        </div>
      </td>
      <td style={{ padding: '16px', textAlign: 'right' }}>
        <strong style={{ fontSize: '16px' }}>${stock.price.toFixed(2)}</strong>
      </td>
      <td style={{ padding: '16px', textAlign: 'right', color: getChangeColor(stock.change) }}>
        {stock.change >= 0 ? '+' : ''}{stock.change.toFixed(2)}
      </td>
      <td style={{ padding: '16px', textAlign: 'right', color: getChangeColor(stock.changePercentage) }}>
        {stock.changePercentage >= 0 ? '+' : ''}{stock.changePercentage.toFixed(2)}%
      </td>
    </tr>
    );
  };

  const StockRow = memo(StockRowComponent);

  return (
    <div style={{ marginTop: '60px' }}>
      <div className="card" style={{ maxWidth: '1000px', width: '100%' }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '24px' }}>
          <h2 style={{ margin: 0 }}>Markets</h2>
          <button
            className="btn btn-secondary"
            onClick={handleRefresh}
            disabled={loading}
            style={{ fontSize: '14px', padding: '8px 16px' }}
            aria-label={loading ? 'Refreshing stock data' : 'Refresh stock data'}
          >
            {loading ? 'Refreshing...' : 'Refresh Data'}
          </button>
        </div>

        {/* Search Bar */}
        <form onSubmit={handleSearch} style={{ marginBottom: '32px' }}>
          <div className="form-group" style={{ marginBottom: '0' }}>
            <label htmlFor="stock-search" className="sr-only">
              Search for stock symbol
            </label>
            <input
              id="stock-search"
              type="text"
              className="form-control"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value.toUpperCase())}
              placeholder="Search by symbol (e.g., AAPL, GOOGL)"
              style={{ fontSize: '16px' }}
              aria-label="Stock symbol search"
              aria-describedby="search-help"
            />
            <small id="search-help" style={{ color: '#666', fontSize: '12px', display: 'block', marginTop: '4px' }}>
              Enter a stock symbol to search
            </small>
          </div>
          <button
            type="submit"
            className="btn btn-primary"
            style={{ width: '100%', marginTop: '12px' }}
            disabled={searchLoading || !searchQuery.trim()}
            aria-label={searchLoading ? 'Searching for stock' : 'Search for stock'}
          >
            {searchLoading ? 'Searching...' : 'Search'}
          </button>
        </form>

        {/* Search Result */}
        {searchResult && (
          <div style={{ marginBottom: '32px' }}>
            {searchResult.error ? (
              <div className="alert alert-error">
                {searchResult.error}
              </div>
            ) : (
              <div style={{ 
                background: '#f8f9fa', 
                padding: '16px', 
                borderRadius: '8px',
                border: '2px solid #667eea'
              }}>
                <h3 style={{ marginBottom: '16px', color: '#333' }}>Search Result</h3>
                <table className="table" style={{ width: '100%', marginBottom: '0' }}>
                  <thead>
                    <tr style={{ background: '#f1f1f1' }}>
                      <th style={{ padding: '12px' }}>Symbol</th>
                      <th style={{ padding: '12px', textAlign: 'right' }}>Price</th>
                      <th style={{ padding: '12px', textAlign: 'right' }}>Change</th>
                      <th style={{ padding: '12px', textAlign: 'right' }}>Change %</th>
                    </tr>
                  </thead>
                  <tbody>
                    <StockRow stock={searchResult} onStockClick={handleStockClick} />
                  </tbody>
                </table>
              </div>
            )}
          </div>
        )}

        {/* Popular Stocks - Memoized to prevent re-renders on unrelated state changes */}
        {useMemo(() => (
          <div style={{ marginBottom: '32px' }}>
            <h3 style={{ marginBottom: '16px', color: '#333' }}>Popular Stocks</h3>
            {loading ? (
              <p style={{ color: '#666', textAlign: 'center', padding: '40px' }}>
                Loading stocks... This may take a moment to avoid rate limiting.
              </p>
            ) : stocks.length === 0 ? (
              <div style={{ 
                background: '#fff3cd', 
                padding: '20px', 
                borderRadius: '8px', 
                border: '1px solid #ffc107',
                textAlign: 'center'
              }}>
                <p style={{ color: '#856404', marginBottom: '8px' }}>
                  Unable to load stock data. This may be due to API rate limiting or network issues.
                </p>
                <p style={{ color: '#856404', fontSize: '14px' }}>
                  Please try refreshing the page in a few moments.
                </p>
              </div>
            ) : (
              <div style={{ overflowX: 'auto' }}>
                <table className="table" style={{ width: '100%', borderCollapse: 'collapse' }}>
                  <thead>
                    <tr style={{ background: '#f1f1f1', textAlign: 'left' }}>
                      <th style={{ padding: '12px' }}>Symbol</th>
                      <th style={{ padding: '12px', textAlign: 'right' }}>Price</th>
                      <th style={{ padding: '12px', textAlign: 'right' }}>Change</th>
                      <th style={{ padding: '12px', textAlign: 'right' }}>Change %</th>
                    </tr>
                  </thead>
                  <tbody>
                    {stocks.map((stock) => (
                      <StockRow key={stock.symbol} stock={stock} onStockClick={handleStockClick} />
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        ), [stocks, loading, StockRow, handleStockClick])}

        {/* Popular Index Funds - Memoized to prevent re-renders on unrelated state changes */}
        {useMemo(() => (
          <div>
            <h3 style={{ marginBottom: '16px', color: '#333' }}>Popular Index Funds</h3>
            {loading ? (
              <p style={{ color: '#666', textAlign: 'center', padding: '40px' }}>
                Loading indexes... This may take a moment to avoid rate limiting.
              </p>
            ) : indexes.length === 0 ? (
              <div style={{ 
                background: '#fff3cd', 
                padding: '20px', 
                borderRadius: '8px', 
                border: '1px solid #ffc107',
                textAlign: 'center'
              }}>
                <p style={{ color: '#856404', marginBottom: '8px' }}>
                  Unable to load index data. This may be due to API rate limiting or network issues.
                </p>
                <p style={{ color: '#856404', fontSize: '14px' }}>
                  Please try refreshing the page in a few moments.
                </p>
              </div>
            ) : (
              <div style={{ overflowX: 'auto' }}>
                <table className="table" style={{ width: '100%', borderCollapse: 'collapse' }}>
                  <thead>
                    <tr style={{ background: '#f1f1f1', textAlign: 'left' }}>
                      <th style={{ padding: '12px' }}>Symbol</th>
                      <th style={{ padding: '12px', textAlign: 'right' }}>Price</th>
                      <th style={{ padding: '12px', textAlign: 'right' }}>Change</th>
                      <th style={{ padding: '12px', textAlign: 'right' }}>Change %</th>
                    </tr>
                  </thead>
                  <tbody>
                    {indexes.map((index) => (
                      <StockRow key={index.symbol} stock={index} onStockClick={handleStockClick} />
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        ), [indexes, loading, StockRow, handleStockClick])}
      </div>

      {/* Trade Modal */}
      <TradeModal
        stock={selectedStock}
        user={user}
        isOpen={isModalOpen}
        onClose={() => setIsModalOpen(false)}
        onTradeSuccess={handleTradeSuccess}
      />
    </div>
  );
};

export default Markets;
