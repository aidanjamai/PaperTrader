import React, { useState, useEffect, useCallback, useMemo, useRef } from 'react';
import { apiRequest } from '../../services/api';
import {
  StockResponse,
  HistoricalDataResponse,
  User,
  BuyStockRequest,
  SellStockRequest,
  UserStock,
} from '../../types';
import { useAuth } from '../../hooks/useAuth';
import { useWatchlist } from '../../hooks/useWatchlist';
import { formatMoney, formatPercent, formatSignedMoney } from '../primitives/format';

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

const POPULAR_STOCKS: StockListing[] = [
  { symbol: 'AAPL', name: 'Apple Inc.', type: 'stock' },
  { symbol: 'GOOGL', name: 'Alphabet Inc.', type: 'stock' },
  { symbol: 'MSFT', name: 'Microsoft Corp.', type: 'stock' },
  { symbol: 'AMZN', name: 'Amazon.com Inc.', type: 'stock' },
  { symbol: 'TSLA', name: 'Tesla, Inc.', type: 'stock' },
  { symbol: 'META', name: 'Meta Platforms Inc.', type: 'stock' },
  { symbol: 'NVDA', name: 'NVIDIA Corp.', type: 'stock' },
  { symbol: 'JPM', name: 'JPMorgan Chase & Co.', type: 'stock' },
  { symbol: 'V', name: 'Visa Inc.', type: 'stock' },
  { symbol: 'JNJ', name: 'Johnson & Johnson', type: 'stock' },
];

const POPULAR_INDEXES: StockListing[] = [
  { symbol: 'SPY', name: 'SPDR S&P 500 ETF', type: 'index' },
  { symbol: 'QQQ', name: 'Invesco QQQ Trust', type: 'index' },
  { symbol: 'DIA', name: 'SPDR Dow Jones Industrial', type: 'index' },
  { symbol: 'IWM', name: 'iShares Russell 2000 ETF', type: 'index' },
  { symbol: 'VTI', name: 'Vanguard Total Stock Market', type: 'index' },
];

const STOCKS_CACHE_KEY = 'papertrader_markets_stocks';
const INDEXES_CACHE_KEY = 'papertrader_markets_indexes';
const CACHE_TIMESTAMP_KEY = 'papertrader_markets_cache_timestamp';

function isCacheValid(timestamp: number): boolean {
  if (!timestamp || timestamp <= 0) return false;
  const now = new Date();
  const cacheDate = new Date(timestamp);
  const isToday = cacheDate.toDateString() === now.toDateString();
  if (!isToday) {
    return (now.getTime() - timestamp) / (1000 * 60 * 60) < 24;
  }
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

interface TradeModalProps {
  stock: StockData | null;
  user: User | null;
  isOpen: boolean;
  onClose: () => void;
  onTradeSuccess: () => void;
}

const TradeModal: React.FC<TradeModalProps> = ({ stock, user, isOpen, onClose, onTradeSuccess }) => {
  const { refreshUser } = useAuth();
  const {
    items: watchlistItems,
    fetchWatchlist,
    addSymbol: addToWatchlist,
    removeSymbol: removeFromWatchlist,
  } = useWatchlist();
  const [tradeType, setTradeType] = useState<'buy' | 'sell'>('buy');
  const [quantity, setQuantity] = useState<string>('');
  const [loading, setLoading] = useState<boolean>(false);
  const [error, setError] = useState<string>('');
  const [message, setMessage] = useState<string>('');
  const [watchlistBusy, setWatchlistBusy] = useState<boolean>(false);
  const [watchlistError, setWatchlistError] = useState<string>('');
  const watchlistFetchedRef = useRef<boolean>(false);

  useEffect(() => {
    if (!isOpen) {
      setQuantity('');
      setError('');
      setMessage('');
      setTradeType('buy');
      setWatchlistError('');
      return;
    }
    if (!watchlistFetchedRef.current) {
      watchlistFetchedRef.current = true;
      fetchWatchlist();
    }
  }, [isOpen, fetchWatchlist]);

  const isWatched = !!stock && watchlistItems.some((e) => e.symbol === stock.symbol);

  const handleWatchlistToggle = async () => {
    if (!stock || watchlistBusy) return;
    setWatchlistError('');
    setWatchlistBusy(true);
    try {
      if (isWatched) {
        await removeFromWatchlist(stock.symbol);
      } else {
        await addToWatchlist(stock.symbol);
      }
    } catch (err) {
      setWatchlistError(err instanceof Error ? err.message : 'Failed to update watchlist');
    } finally {
      setWatchlistBusy(false);
    }
  };

  const handleSubmit = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    if (!stock || !user) return;

    setError('');
    setMessage('');
    setLoading(true);

    const quantityNum = parseInt(quantity, 10);
    if (isNaN(quantityNum) || quantityNum <= 0) {
      setError('Quantity must be a positive whole number.');
      setLoading(false);
      return;
    }

    try {
      const endpoint = tradeType === 'buy' ? '/investments/buy' : '/investments/sell';
      const body: BuyStockRequest | SellStockRequest = {
        symbol: stock.symbol,
        quantity: quantityNum,
      };

      const response = await apiRequest<UserStock>(endpoint, {
        method: 'POST',
        body: JSON.stringify(body),
      });

      if (response.ok) {
        await response.json();
        setMessage(
          `${tradeType === 'buy' ? 'Bought' : 'Sold'} ${quantityNum} ${stock.symbol}.`
        );
        await refreshUser();
        setQuantity('');
        setTimeout(() => {
          onTradeSuccess();
          onClose();
        }, 1200);
      } else {
        const txt = await response.text();
        try {
          const j = JSON.parse(txt) as { message?: string };
          setError(j.message || txt);
        } catch {
          setError(txt || `${tradeType === 'buy' ? 'Buy' : 'Sell'} failed.`);
        }
      }
    } catch {
      setError('Network error. Please try again.');
    } finally {
      setLoading(false);
    }
  };

  if (!isOpen || !stock || !user) return null;

  const quantityNum = parseInt(quantity, 10) || 0;
  const totalCost = quantityNum * stock.price;
  const remainingBalance =
    tradeType === 'buy' ? user.balance - totalCost : user.balance + totalCost;
  const insufficient = remainingBalance < 0 && tradeType === 'buy' && quantityNum > 0;
  const tone: 'gain' | 'loss' | 'flat' =
    stock.change > 0 ? 'gain' : stock.change < 0 ? 'loss' : 'flat';

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal-content" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <div>
            <div className="eyebrow" style={{ marginBottom: 4 }}>
              {stock.symbol}
            </div>
            <h2>{stock.name}</h2>
          </div>
          <button type="button" className="modal-close" onClick={onClose} aria-label="Close">
            ×
          </button>
        </div>

        <div className="modal-body">
          <div
            style={{
              padding: '16px 18px',
              border: '1px solid var(--hairline)',
              borderRadius: 8,
              marginBottom: 20,
              background: 'var(--canvas)',
            }}
          >
            <div className="eyebrow" style={{ marginBottom: 6 }}>
              Last price
            </div>
            <div className="display-num" style={{ fontSize: 36 }}>
              <span className="currency">$</span>
              <span className="num">{formatMoney(stock.price)}</span>
            </div>
            <div className={`change-line ${tone}`} style={{ fontSize: 12 }}>
              <span>{formatSignedMoney(stock.change)}</span>
              <span>({formatPercent(stock.changePercentage)})</span>
            </div>
          </div>

          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              gap: 12,
              padding: '10px 14px',
              border: '1px solid var(--hairline)',
              borderRadius: 6,
              marginBottom: 16,
            }}
          >
            <div>
              <div style={{ fontSize: 13, color: 'var(--ink)' }}>Watchlist</div>
              <div style={{ fontSize: 11, color: 'var(--ink-muted)' }}>
                {isWatched
                  ? `Tracking ${stock.symbol} on your dashboard.`
                  : 'Track this symbol without holding shares.'}
              </div>
            </div>
            <button
              type="button"
              role="switch"
              aria-checked={isWatched}
              aria-label={isWatched ? `Remove ${stock.symbol} from watchlist` : `Add ${stock.symbol} to watchlist`}
              onClick={handleWatchlistToggle}
              disabled={watchlistBusy}
              style={{
                position: 'relative',
                width: 40,
                height: 22,
                padding: 0,
                border: '1px solid var(--hairline)',
                borderRadius: 999,
                background: isWatched ? 'var(--accent, var(--ink))' : 'var(--canvas)',
                cursor: watchlistBusy ? 'wait' : 'pointer',
                transition: 'background 120ms ease',
                opacity: watchlistBusy ? 0.6 : 1,
                flexShrink: 0,
              }}
            >
              <span
                aria-hidden="true"
                style={{
                  position: 'absolute',
                  top: 2,
                  left: isWatched ? 20 : 2,
                  width: 16,
                  height: 16,
                  borderRadius: '50%',
                  background: isWatched ? 'var(--surface)' : 'var(--ink-muted)',
                  transition: 'left 120ms ease, background 120ms ease',
                }}
              />
            </button>
          </div>
          {watchlistError && (
            <div className="alert alert-error" style={{ marginTop: -8 }}>
              {watchlistError}
            </div>
          )}

          <div className="form-group">
            <label className="form-label">Side</label>
            <div style={{ display: 'flex', gap: 8 }}>
              <button
                type="button"
                className={`btn ${tradeType === 'buy' ? 'btn-primary' : 'btn-secondary'}`}
                style={{ flex: 1 }}
                onClick={() => setTradeType('buy')}
              >
                Buy
              </button>
              <button
                type="button"
                className={`btn ${tradeType === 'sell' ? 'btn-primary' : 'btn-secondary'}`}
                style={{ flex: 1 }}
                onClick={() => setTradeType('sell')}
              >
                Sell
              </button>
            </div>
          </div>

          {error && <div className="alert alert-error">{error}</div>}
          {message && <div className="alert alert-success">{message}</div>}

          <form onSubmit={handleSubmit}>
            <div className="form-group">
              <label htmlFor="modal-quantity" className="form-label">
                Quantity
              </label>
              <input
                id="modal-quantity"
                type="number"
                className="input mono"
                value={quantity}
                onChange={(e) => setQuantity(e.target.value)}
                placeholder="0"
                required
                disabled={loading}
                min="1"
                step="1"
              />
              <div className="form-hint">Whole shares only.</div>
            </div>

            {quantityNum > 0 && (
              <div
                className="mono"
                style={{
                  fontSize: 13,
                  background: 'var(--canvas)',
                  border: '1px solid var(--hairline)',
                  borderRadius: 4,
                  padding: '12px 14px',
                  marginBottom: 16,
                  display: 'grid',
                  rowGap: 6,
                }}
              >
                <Row label="Shares" value={String(quantityNum)} />
                <Row label="Price / share" value={`$${formatMoney(stock.price)}`} />
                <Row
                  label={tradeType === 'buy' ? 'Total cost' : 'Total proceeds'}
                  value={`$${formatMoney(totalCost)}`}
                  emphasis
                />
                <Row label="Cash before" value={`$${formatMoney(user.balance)}`} />
                <Row
                  label="Cash after"
                  value={`$${formatMoney(remainingBalance)}`}
                  emphasis
                  tone={insufficient ? 'loss' : undefined}
                />
                {insufficient && (
                  <div className="loss-text" style={{ marginTop: 4 }}>
                    Insufficient funds.
                  </div>
                )}
              </div>
            )}

            <button
              type="submit"
              className="btn btn-primary btn-block"
              disabled={loading || insufficient}
            >
              {loading
                ? `${tradeType === 'buy' ? 'Buying' : 'Selling'}…`
                : `${tradeType === 'buy' ? 'Buy' : 'Sell'} ${stock.symbol}`}
            </button>
          </form>
        </div>
      </div>
    </div>
  );
};

const Row: React.FC<{
  label: string;
  value: string;
  emphasis?: boolean;
  tone?: 'gain' | 'loss';
}> = ({ label, value, emphasis, tone }) => (
  <div style={{ display: 'flex', justifyContent: 'space-between' }}>
    <span style={{ color: 'var(--ink-muted)' }}>{label}</span>
    <span
      style={{
        color: tone === 'loss' ? 'var(--loss)' : tone === 'gain' ? 'var(--gain)' : 'var(--ink)',
        fontWeight: emphasis ? 500 : 400,
      }}
    >
      {value}
    </span>
  </div>
);

const StockTable: React.FC<{
  rows: StockData[];
  onRowClick: (s: StockData) => void;
}> = ({ rows, onRowClick }) => (
  <table className="holdings">
    <thead>
      <tr>
        <th>Symbol</th>
        <th className="num">Price</th>
        <th className="num">Change</th>
        <th className="num">Change %</th>
      </tr>
    </thead>
    <tbody>
      {rows.map((s) => {
        const pillTone: 'gain' | 'loss' | null =
          s.change > 0 ? 'gain' : s.change < 0 ? 'loss' : null;
        return (
          <tr
            key={s.symbol}
            role="button"
            tabIndex={0}
            style={{ cursor: 'pointer' }}
            onClick={() => onRowClick(s)}
            onKeyDown={(e) => {
              if (e.key === 'Enter' || e.key === ' ') {
                e.preventDefault();
                onRowClick(s);
              }
            }}
          >
            <td>
              <div className="ticker">
                <span className="mark" aria-hidden="true">
                  {s.symbol.charAt(0)}
                </span>
                <span className="sym">{s.symbol}</span>
                <span className="name">{s.name}</span>
              </div>
            </td>
            <td className="num">${formatMoney(s.price)}</td>
            <td className="num">
              {pillTone ? (
                <span className={`pill pill-${pillTone}`}>{formatSignedMoney(s.change)}</span>
              ) : (
                formatSignedMoney(s.change)
              )}
            </td>
            <td className="num">
              {pillTone ? (
                <span className={`pill pill-${pillTone}`}>{formatPercent(s.changePercentage)}</span>
              ) : (
                formatPercent(s.changePercentage)
              )}
            </td>
          </tr>
        );
      })}
    </tbody>
  </table>
);

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

  const loadCachedData = useCallback((): CachedData | null => {
    try {
      const cs = localStorage.getItem(STOCKS_CACHE_KEY);
      const ci = localStorage.getItem(INDEXES_CACHE_KEY);
      const ct = localStorage.getItem(CACHE_TIMESTAMP_KEY);
      if (!cs || !ci || !ct) return null;
      const t = parseInt(ct, 10);
      if (!isCacheValid(t)) return null;
      const sd = JSON.parse(cs) as StockData[];
      const id = JSON.parse(ci) as StockData[];
      if (!Array.isArray(sd) || !Array.isArray(id)) return null;
      if (sd.length === 0 && id.length === 0) return null;
      return { stocks: sd, indexes: id, timestamp: t };
    } catch {
      return null;
    }
  }, []);

  const saveCachedData = useCallback((sd: StockData[], id: StockData[]) => {
    try {
      localStorage.setItem(STOCKS_CACHE_KEY, JSON.stringify(sd));
      localStorage.setItem(INDEXES_CACHE_KEY, JSON.stringify(id));
      localStorage.setItem(CACHE_TIMESTAMP_KEY, Date.now().toString());
    } catch {
      // ignore
    }
  }, []);

  const fetchBatchStockData = useCallback(
    async (symbols: string[]): Promise<Map<string, StockData>> => {
      const result = new Map<string, StockData>();
      if (symbols.length === 0) return result;

      try {
        const encoded = symbols.map((s) => encodeURIComponent(s)).join(',');
        const response = await apiRequest<{ [s: string]: HistoricalDataResponse }>(
          `/market/stock/historical/daily/batch?symbols=${encoded}`
        );
        if (!response.ok) return result;

        const responseData = (await response.json()) as {
          success: boolean;
          message: string;
          data: { [s: string]: HistoricalDataResponse };
        };
        const batchData =
          responseData.data ||
          (responseData as unknown as { [s: string]: HistoricalDataResponse });

        for (const symbol of symbols) {
          const h = batchData[symbol];
          if (!h || h.price === undefined) continue;
          const listing = [...POPULAR_STOCKS, ...POPULAR_INDEXES].find((s) => s.symbol === symbol);
          result.set(symbol, {
            symbol: h.symbol,
            name: listing?.name || symbol,
            type: listing?.type || 'stock',
            price: h.price,
            change: h.change || 0,
            changePercentage: h.change_percentage || 0,
            date: h.date,
          });
        }
      } catch {
        // swallow — caller handles empty result
      }
      return result;
    },
    []
  );

  const fetchStockData = useCallback(async (symbol: string): Promise<StockData | null> => {
    try {
      const historicalResponse = await apiRequest<HistoricalDataResponse>(
        `/market/stock/historical/daily?symbol=${symbol}`
      );

      if (!historicalResponse.ok) {
        try {
          const priceResponse = await apiRequest<StockResponse>(
            `/market/stock?symbol=${symbol}`
          );
          if (!priceResponse.ok) return null;
          const pdr = (await priceResponse.json()) as {
            success: boolean;
            message: string;
            data: StockResponse;
          };
          const pd = pdr.data || (pdr as unknown as StockResponse);
          const listing = [...POPULAR_STOCKS, ...POPULAR_INDEXES].find((s) => s.symbol === symbol);
          return {
            symbol: pd.symbol,
            name: listing?.name || symbol,
            type: listing?.type || 'stock',
            price: pd.price,
            change: 0,
            changePercentage: 0,
            date: pd.date,
          };
        } catch {
          return null;
        }
      }

      let h: HistoricalDataResponse;
      try {
        const hdr = (await historicalResponse.json()) as {
          success: boolean;
          message: string;
          data: HistoricalDataResponse;
        };
        h = hdr.data || (hdr as unknown as HistoricalDataResponse);
      } catch {
        return null;
      }
      if (!h || h.price === undefined) return null;

      const listing = [...POPULAR_STOCKS, ...POPULAR_INDEXES].find((s) => s.symbol === symbol);
      return {
        symbol: h.symbol,
        name: listing?.name || symbol,
        type: listing?.type || 'stock',
        price: h.price,
        change: h.change || 0,
        changePercentage: h.change_percentage || 0,
        date: h.date,
      };
    } catch {
      return null;
    }
  }, []);

  const processBatchData = useCallback((batch: Map<string, StockData>) => {
    const stockResults: StockData[] = [];
    const indexResults: StockData[] = [];
    for (const s of POPULAR_STOCKS) {
      const d = batch.get(s.symbol);
      if (d && d.price !== undefined) stockResults.push({ ...s, ...d });
    }
    for (const i of POPULAR_INDEXES) {
      const d = batch.get(i.symbol);
      if (d && d.price !== undefined) indexResults.push({ ...i, ...d });
    }
    return { stockResults, indexResults };
  }, []);

  const fetchAndProcessStocks = useCallback(
    async (forceRefresh: boolean = false) => {
      if (!forceRefresh) {
        const cached = loadCachedData();
        if (cached) {
          const hs = cached.stocks && cached.stocks.length > 0;
          const hi = cached.indexes && cached.indexes.length > 0;
          if (hs || hi) {
            setStocks(cached.stocks || []);
            setIndexes(cached.indexes || []);
            setLoading(false);
            return;
          }
          localStorage.removeItem(STOCKS_CACHE_KEY);
          localStorage.removeItem(INDEXES_CACHE_KEY);
          localStorage.removeItem(CACHE_TIMESTAMP_KEY);
        }
      }

      const allSymbols = [
        ...POPULAR_STOCKS.map((s) => s.symbol),
        ...POPULAR_INDEXES.map((s) => s.symbol),
      ];
      const batch = await fetchBatchStockData(allSymbols);
      const { stockResults, indexResults } = processBatchData(batch);
      setStocks(stockResults);
      setIndexes(indexResults);
      if (stockResults.length > 0 || indexResults.length > 0) {
        saveCachedData(stockResults, indexResults);
      }
      setLoading(false);
    },
    [fetchBatchStockData, loadCachedData, saveCachedData, processBatchData]
  );

  useEffect(() => {
    let mounted = true;
    setLoading(true);
    fetchAndProcessStocks(false).catch(() => {
      if (mounted) setLoading(false);
    });
    return () => {
      mounted = false;
    };
  }, [fetchAndProcessStocks]);

  const handleRefresh = useCallback(async () => {
    localStorage.removeItem(STOCKS_CACHE_KEY);
    localStorage.removeItem(INDEXES_CACHE_KEY);
    localStorage.removeItem(CACHE_TIMESTAMP_KEY);
    setLoading(true);
    try {
      await fetchAndProcessStocks(true);
    } catch {
      setLoading(false);
    }
  }, [fetchAndProcessStocks]);

  const handleSearch = useCallback(
    async (e: React.FormEvent<HTMLFormElement>) => {
      e.preventDefault();
      const trimmed = searchQuery.trim();
      if (!trimmed) return;

      setSearchLoading(true);
      setSearchResult(null);
      const symbol = trimmed.toUpperCase().replace(/[^A-Z.]/g, '');
      if (!symbol) {
        setSearchResult({
          symbol: trimmed,
          name: trimmed,
          type: 'stock',
          price: 0,
          change: 0,
          changePercentage: 0,
          date: '',
          error: 'Invalid symbol format',
        });
        setSearchLoading(false);
        return;
      }

      const cached = [...stocks, ...indexes].find((s) => s.symbol === symbol);
      if (cached) {
        setSearchResult(cached);
        setSearchLoading(false);
        return;
      }

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
            error: 'Stock not found',
          });
        }
      } catch {
        setSearchResult({
          symbol,
          name: symbol,
          type: 'stock',
          price: 0,
          change: 0,
          changePercentage: 0,
          date: '',
          error: 'Error fetching stock data. Please try again.',
        });
      } finally {
        setSearchLoading(false);
      }
    },
    [searchQuery, stocks, indexes, fetchStockData]
  );

  const handleStockClick = useCallback(
    (stock: StockData) => {
      if (!isAuthenticated || !user) {
        setSearchResult({ ...stock, error: 'Please log in to trade.' });
        return;
      }
      if (!user.email_verified) {
        setSearchResult({
          ...stock,
          error: 'Verify your email to trade — see the banner at the top of the page.',
        });
        return;
      }
      setSelectedStock(stock);
      setIsModalOpen(true);
    },
    [isAuthenticated, user]
  );

  const handleTradeSuccess = useCallback(() => {
    setIsModalOpen(false);
  }, []);

  const popularStocksTable = useMemo(
    () => <StockTable rows={stocks} onRowClick={handleStockClick} />,
    [stocks, handleStockClick]
  );

  const popularIndexesTable = useMemo(
    () => <StockTable rows={indexes} onRowClick={handleStockClick} />,
    [indexes, handleStockClick]
  );

  return (
    <div className="container">
      <header className="page-header">
        <div>
          <div className="eyebrow" style={{ marginBottom: 6 }}>
            Universe · live prices
          </div>
          <h1>Markets</h1>
        </div>
        <div className="meta">
          <button
            type="button"
            className="btn btn-secondary btn-sm"
            onClick={handleRefresh}
            disabled={loading}
          >
            {loading ? 'Refreshing…' : 'Refresh'}
          </button>
        </div>
      </header>

      <form onSubmit={handleSearch} style={{ marginBottom: 24 }}>
        <div style={{ display: 'flex', gap: 8 }}>
          <input
            id="stock-search"
            type="text"
            className="input mono"
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value.toUpperCase())}
            placeholder="Search symbol — AAPL, GOOGL, MSFT…"
            aria-label="Stock symbol search"
            spellCheck={false}
            autoComplete="off"
          />
          <button
            type="submit"
            className="btn btn-primary"
            disabled={searchLoading || !searchQuery.trim()}
          >
            {searchLoading ? 'Searching…' : 'Search'}
          </button>
        </div>
      </form>

      {searchResult && (
        <div style={{ marginBottom: 24 }}>
          {searchResult.error ? (
            <div className="alert alert-error">{searchResult.error}</div>
          ) : (
            <div className="panel">
              <div className="panel-head">
                <h3>Search result</h3>
                <button
                  type="button"
                  className="btn btn-ghost btn-sm"
                  onClick={() => setSearchResult(null)}
                >
                  Clear
                </button>
              </div>
              <StockTable rows={[searchResult]} onRowClick={handleStockClick} />
            </div>
          )}
        </div>
      )}

      <section className="panel" style={{ marginBottom: 24 }}>
        <div className="panel-head">
          <h3>Popular stocks</h3>
          <span className="mono" style={{ color: 'var(--ink-muted)', fontSize: 11 }}>
            {stocks.length}/{POPULAR_STOCKS.length} loaded
          </span>
        </div>
        {loading ? (
          <div className="empty-state">Loading popular stocks…</div>
        ) : stocks.length === 0 ? (
          <div className="empty-state">
            <span className="empty-title">Unable to load stock data</span>
            <span>API may be rate-limited. Try refreshing in a moment.</span>
          </div>
        ) : (
          popularStocksTable
        )}
      </section>

      <section className="panel">
        <div className="panel-head">
          <h3>Popular index funds</h3>
          <span className="mono" style={{ color: 'var(--ink-muted)', fontSize: 11 }}>
            {indexes.length}/{POPULAR_INDEXES.length} loaded
          </span>
        </div>
        {loading ? (
          <div className="empty-state">Loading indexes…</div>
        ) : indexes.length === 0 ? (
          <div className="empty-state">
            <span className="empty-title">Unable to load index data</span>
            <span>API may be rate-limited. Try refreshing in a moment.</span>
          </div>
        ) : (
          popularIndexesTable
        )}
      </section>

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
