import React, { useState, FormEvent, ChangeEvent, useEffect, useMemo, useRef } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { apiRequest } from '../../services/api';
import {
  TradeAction,
  BuyStockRequest,
  SellStockRequest,
  UserStock,
  StockResponse,
} from '../../types';
import { useAuth } from '../../hooks/useAuth';
import { formatMoney } from '../primitives/format';

type QuoteState =
  | { status: 'idle' }
  | { status: 'loading'; symbol: string }
  | { status: 'ready'; symbol: string; price: number; date: string }
  | { status: 'error'; symbol: string; message: string };

const formatQuoteDate = (raw: string): string => {
  const match = /^(\d{2})\/(\d{2})\/(\d{4})$/.exec(raw);
  if (!match) return raw;
  const [, mm, dd, yyyy] = match;
  const d = new Date(Number(yyyy), Number(mm) - 1, Number(dd));
  if (Number.isNaN(d.getTime())) return raw;
  return d.toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
  });
};

const Trade: React.FC = () => {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const { user, refreshUser } = useAuth();
  const [tradeType, setTradeType] = useState<TradeAction>('buy');
  // Allow stock-detail / dashboard links to prefill the symbol via ?symbol=AAPL.
  const [symbol, setSymbol] = useState<string>(
    (searchParams.get('symbol') || '').toUpperCase()
  );
  const [quantity, setQuantity] = useState<string>('');
  const [loading, setLoading] = useState<boolean>(false);
  const [message, setMessage] = useState<string>('');
  const [error, setError] = useState<string>('');

  const quantityNum = useMemo(() => {
    const n = parseInt(quantity, 10);
    return Number.isFinite(n) && n > 0 ? n : 0;
  }, [quantity]);

  const trimmedSymbol = symbol.toUpperCase().trim();
  const [quote, setQuote] = useState<QuoteState>({ status: 'idle' });

  const inflightRef = useRef<AbortController | null>(null);
  useEffect(() => {
    if (!trimmedSymbol) {
      setQuote({ status: 'idle' });
      return;
    }

    const handle = window.setTimeout(() => {
      inflightRef.current?.abort();
      const ctrl = new AbortController();
      inflightRef.current = ctrl;
      setQuote({ status: 'loading', symbol: trimmedSymbol });

      apiRequest<StockResponse>(`/market/stock?symbol=${encodeURIComponent(trimmedSymbol)}`, {
        signal: ctrl.signal,
      })
        .then(async (res) => {
          if (ctrl.signal.aborted) return;
          if (!res.ok) {
            const message =
              res.status === 400
                ? 'Invalid symbol.'
                : res.status === 404
                ? 'Symbol not found.'
                : res.status === 429
                ? 'Too many lookups — try again in a moment.'
                : 'Could not fetch a price.';
            setQuote({ status: 'error', symbol: trimmedSymbol, message });
            return;
          }
          const body = (await res.json()) as
            | { data?: StockResponse; success?: boolean }
            | StockResponse;
          const data: StockResponse =
            (body as { data?: StockResponse }).data ?? (body as StockResponse);
          if (!data || typeof data.price !== 'number') {
            setQuote({
              status: 'error',
              symbol: trimmedSymbol,
              message: 'Could not fetch a price.',
            });
            return;
          }
          setQuote({
            status: 'ready',
            symbol: data.symbol || trimmedSymbol,
            price: data.price,
            date: data.date,
          });
        })
        .catch((err: unknown) => {
          if ((err as { name?: string })?.name === 'AbortError') return;
          setQuote({
            status: 'error',
            symbol: trimmedSymbol,
            message: 'Network error fetching price.',
          });
        });
    }, 400);

    return () => {
      window.clearTimeout(handle);
      inflightRef.current?.abort();
    };
  }, [trimmedSymbol]);

  const handleSubmit = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    setError('');
    setMessage('');

    if (!symbol.trim()) {
      setError('Please enter a stock symbol.');
      return;
    }
    if (quantityNum <= 0) {
      setError('Quantity must be a positive whole number.');
      return;
    }

    setLoading(true);
    try {
      const endpoint = tradeType === 'buy' ? '/investments/buy' : '/investments/sell';
      const requestBody: BuyStockRequest | SellStockRequest = {
        symbol: symbol.toUpperCase().trim(),
        quantity: quantityNum,
      };

      const response = await apiRequest<UserStock>(endpoint, {
        method: 'POST',
        body: JSON.stringify(requestBody),
      });

      if (response.ok) {
        await response.json();
        setMessage(
          `${tradeType === 'buy' ? 'Bought' : 'Sold'} ${quantityNum} shares of ${symbol
            .toUpperCase()
            .trim()}.`
        );
        await refreshUser();
        setSymbol('');
        setQuantity('');
      } else {
        const errorData = await response.text();
        try {
          const jsonError = JSON.parse(errorData) as { message?: string };
          setError(jsonError.message || errorData);
        } catch {
          setError(errorData || `${tradeType === 'buy' ? 'Buy' : 'Sell'} failed.`);
        }
      }
    } catch {
      setError('Network error. Please try again.');
    } finally {
      setLoading(false);
    }
  };

  if (!user || !user.email_verified) {
    return (
      <div className="container-narrow" style={{ paddingTop: 40 }}>
        <div className="panel" style={{ padding: 28 }}>
          <div className="eyebrow" style={{ marginBottom: 6 }}>
            Verify · required
          </div>
          <h2
            className="serif"
            style={{ margin: '0 0 12px 0', fontWeight: 400, fontSize: 28, letterSpacing: '-0.01em' }}
          >
            Verify your email to trade.
          </h2>
          <p className="muted" style={{ margin: '0 0 20px 0', maxWidth: 520 }}>
            Trading is locked until your email is verified. Check the banner at the top of the
            page or your inbox for the verification link.
          </p>
          <button type="button" className="btn btn-secondary" onClick={() => navigate('/markets')}>
            Browse markets
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="container-narrow" style={{ paddingTop: 24 }}>
      <header className="page-header">
        <div>
          <div className="eyebrow" style={{ marginBottom: 6 }}>
            Action · order ticket
          </div>
          <h1>Trade</h1>
        </div>
        <div className="meta">
          Cash <span className="accent-when">${formatMoney(user.balance ?? 0)}</span>
        </div>
      </header>

      <div className="panel" style={{ padding: 28 }}>
        {error && <div className="alert alert-error">{error}</div>}
        {message && <div className="alert alert-success">{message}</div>}

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

        <form onSubmit={handleSubmit}>
          <div className="form-group">
            <label htmlFor="symbol" className="form-label">
              Symbol
            </label>
            <input
              type="text"
              id="symbol"
              className="input mono"
              value={symbol}
              onChange={(e: ChangeEvent<HTMLInputElement>) =>
                setSymbol(e.target.value.toUpperCase())
              }
              placeholder="AAPL"
              required
              disabled={loading}
              maxLength={10}
              autoComplete="off"
              spellCheck={false}
            />
          </div>

          <div className="form-group">
            <label htmlFor="quantity" className="form-label">
              Quantity
            </label>
            <input
              type="number"
              id="quantity"
              className="input mono"
              value={quantity}
              onChange={(e: ChangeEvent<HTMLInputElement>) => setQuantity(e.target.value)}
              placeholder="0"
              required
              disabled={loading}
              min="1"
              step="1"
            />
            <div className="form-hint">Whole shares only.</div>
          </div>

          <div
            className="form-group"
            aria-live="polite"
            style={{
              border: '1px solid var(--hairline)',
              borderRadius: 6,
              padding: '14px 16px',
              background: 'transparent',
            }}
          >
            <div
              className="eyebrow"
              style={{ marginBottom: 10, display: 'flex', justifyContent: 'space-between' }}
            >
              <span>Preview</span>
              {quote.status === 'ready' && quote.date && (
                <span className="muted" style={{ textTransform: 'none', letterSpacing: 0 }}>
                  as of {formatQuoteDate(quote.date)}
                </span>
              )}
            </div>

            {quote.status === 'idle' && (
              <div className="muted" style={{ fontSize: 13 }}>
                Enter a symbol to see the latest price.
              </div>
            )}

            {quote.status === 'loading' && (
              <div className="muted" style={{ fontSize: 13 }}>
                Fetching {quote.symbol}…
              </div>
            )}

            {quote.status === 'error' && (
              <div className="loss-text" style={{ fontSize: 13 }}>
                {quote.message}
              </div>
            )}

            {quote.status === 'ready' && (
              <div style={{ display: 'grid', gap: 8 }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline' }}>
                  <span className="muted" style={{ fontSize: 13 }}>
                    {quote.symbol} price
                  </span>
                  <span className="mono" style={{ fontSize: 18 }}>
                    ${formatMoney(quote.price)}
                  </span>
                </div>
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline' }}>
                  <span className="muted" style={{ fontSize: 13 }}>
                    Estimated {tradeType === 'buy' ? 'cost' : 'proceeds'} ({quantityNum > 0 ? quantityNum : 0} ×)
                  </span>
                  <span className="mono" style={{ fontSize: 18 }}>
                    ${formatMoney(quote.price * quantityNum)}
                  </span>
                </div>
                {tradeType === 'buy' &&
                  quantityNum > 0 &&
                  quote.price * quantityNum > (user.balance ?? 0) && (
                    <div className="form-hint loss-text">
                      Estimated cost exceeds your cash balance of ${formatMoney(user.balance ?? 0)}.
                    </div>
                  )}
                <div className="form-hint">
                  Final fill price comes from the live market at submit and may differ.
                </div>
              </div>
            )}
          </div>

          <button
            type="submit"
            className="btn btn-primary btn-lg btn-block"
            disabled={loading}
          >
            {loading
              ? `${tradeType === 'buy' ? 'Buying' : 'Selling'}…`
              : `${tradeType === 'buy' ? 'Buy' : 'Sell'} ${symbol || 'shares'}`}
          </button>
        </form>
      </div>
    </div>
  );
};

export default Trade;
