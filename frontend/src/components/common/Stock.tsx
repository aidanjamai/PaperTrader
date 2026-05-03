import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { apiRequest } from '../../services/api';
import {
  HistoricalSeriesResponse,
  StockResponse,
  WatchlistEntry,
} from '../../types';
import { useAuth } from '../../hooks/useAuth';
import { formatMoney, formatPercent, formatSignedMoney } from '../primitives/format';
import PriceChart from './PriceChart';

// Matches util.ValidateSymbol on the backend (1–10 letters, optional .CC suffix).
const SYMBOL_PATTERN = /^[A-Z]{1,10}(\.[A-Z]{1,2})?$/;

type RangeKey = '1M' | '3M' | 'YTD' | '1Y';
type Range = { key: RangeKey; days: number; label: string };

// YTD's `days` is computed at render so it stays accurate across the year.
const ytdDays = (): number => {
  const now = new Date();
  const yearStart = new Date(now.getFullYear(), 0, 1);
  const elapsed = Math.floor((now.getTime() - yearStart.getTime()) / 86_400_000);
  // Floor at 7 to satisfy the backend's minSeriesDays clamp early in January.
  return Math.max(7, elapsed);
};

const buildRanges = (): Range[] => [
  { key: '1M', days: 30, label: '1M' },
  { key: '3M', days: 90, label: '3M' },
  { key: 'YTD', days: ytdDays(), label: 'YTD' },
  { key: '1Y', days: 365, label: '1Y' },
];

const Stock: React.FC = () => {
  const { symbol: rawSymbol } = useParams<{ symbol: string }>();
  const navigate = useNavigate();
  const { user } = useAuth();

  const symbol = (rawSymbol || '').toUpperCase();
  // Mirror the validation server-side at util.ValidateSymbol so we surface a
  // local error before firing a doomed network request.
  const symbolValid = SYMBOL_PATTERN.test(symbol);

  // Built once per render — buildRanges is cheap and YTD's day count needs to
  // stay accurate as the calendar advances; useMemo with [] would freeze it.
  const ranges = buildRanges();
  const [range, setRange] = useState<Range>(ranges[1]); // default to 3M
  const [series, setSeries] = useState<HistoricalSeriesResponse | null>(null);
  const [seriesLoading, setSeriesLoading] = useState(true);
  const [seriesError, setSeriesError] = useState<string | null>(null);
  const [quote, setQuote] = useState<StockResponse | null>(null);
  const [quoteLoading, setQuoteLoading] = useState(true);
  const [quoteError, setQuoteError] = useState<string | null>(null);
  const [watchAdding, setWatchAdding] = useState(false);
  const [watchMessage, setWatchMessage] = useState<{ kind: 'ok' | 'err'; text: string } | null>(null);

  // Note: range stats (high/low/change) are computed client-side in JS doubles.
  // For prices ≤ ~$10K this is exact at 2dp; the backend stays the source of
  // truth for the underlying decimal closes.
  const fetchQuote = useCallback(async () => {
    if (!symbol || !symbolValid) {
      setQuoteLoading(false);
      return;
    }
    setQuoteLoading(true);
    setQuoteError(null);
    try {
      const res = await apiRequest<{ data?: StockResponse }>(
        `/market/stock?symbol=${encodeURIComponent(symbol)}`
      );
      if (!res.ok) {
        setQuoteError(res.status === 404 ? 'Symbol not found.' : 'Could not fetch a price.');
        return;
      }
      const body = (await res.json()) as { data?: StockResponse } | StockResponse;
      const data = (body as { data?: StockResponse }).data ?? (body as StockResponse);
      setQuote(data ?? null);
    } catch {
      setQuoteError('Network error fetching price.');
    } finally {
      setQuoteLoading(false);
    }
  }, [symbol]);

  const fetchSeries = useCallback(async (signal?: AbortSignal) => {
    if (!symbol || !symbolValid) {
      setSeriesLoading(false);
      return;
    }
    setSeriesLoading(true);
    setSeriesError(null);
    try {
      const res = await apiRequest<{ data?: HistoricalSeriesResponse }>(
        `/market/stock/historical/series?symbol=${encodeURIComponent(symbol)}&days=${range.days}`,
        { signal }
      );
      if (signal?.aborted) return;
      if (!res.ok) {
        setSeriesError(
          res.status === 404
            ? 'No historical data for this symbol.'
            : `Couldn't load chart (${res.status}).`
        );
        return;
      }
      const body = (await res.json()) as
        | { data?: HistoricalSeriesResponse }
        | HistoricalSeriesResponse;
      const data = (body as { data?: HistoricalSeriesResponse }).data ?? (body as HistoricalSeriesResponse);
      if (signal?.aborted) return;
      setSeries(data ?? null);
    } catch (err) {
      if ((err as { name?: string })?.name === 'AbortError') return;
      setSeriesError('Network error loading chart.');
    } finally {
      if (!signal?.aborted) setSeriesLoading(false);
    }
  }, [symbol, symbolValid, range.days]);

  useEffect(() => {
    fetchQuote();
  }, [fetchQuote]);

  useEffect(() => {
    const ctrl = new AbortController();
    fetchSeries(ctrl.signal);
    return () => ctrl.abort();
  }, [fetchSeries]);

  const rangeStats = useMemo(() => {
    if (!series || series.points.length < 2) return null;
    const first = series.points[0].close;
    const last = series.points[series.points.length - 1].close;
    const change = last - first;
    const changePct = first > 0 ? (change / first) * 100 : 0;
    const high = Math.max(...series.points.map((p) => p.close));
    const low = Math.min(...series.points.map((p) => p.close));
    return { first, last, change, changePct, high, low };
  }, [series]);

  const handleAddToWatchlist = async () => {
    setWatchAdding(true);
    setWatchMessage(null);
    try {
      const res = await apiRequest<WatchlistEntry>('/watchlist', {
        method: 'POST',
        body: JSON.stringify({ symbol }),
      });
      if (res.ok) {
        setWatchMessage({ kind: 'ok', text: 'Added to watchlist.' });
      } else if (res.status === 409) {
        setWatchMessage({ kind: 'ok', text: 'Already in your watchlist.' });
      } else {
        const body = await res.json().catch(() => null as unknown);
        const msg =
          (body && typeof body === 'object' && 'message' in body
            ? String((body as { message: unknown }).message)
            : null) || 'Could not add to watchlist.';
        setWatchMessage({ kind: 'err', text: msg });
      }
    } catch {
      setWatchMessage({ kind: 'err', text: 'Network error.' });
    } finally {
      setWatchAdding(false);
    }
  };

  const trend: 'gain' | 'loss' | 'flat' =
    rangeStats == null
      ? 'flat'
      : rangeStats.change > 0
      ? 'gain'
      : rangeStats.change < 0
      ? 'loss'
      : 'flat';

  return (
    <div className="container">
      <header className="page-header">
        <div>
          <div className="eyebrow" style={{ marginBottom: 6 }}>
            Symbol · daily close
          </div>
          <h1 style={{ display: 'flex', alignItems: 'baseline', gap: 12 }}>
            <span className="ticker" style={{ fontSize: 'inherit' }}>
              <span className="mark" aria-hidden="true">
                {symbol.charAt(0)}
              </span>
              <span className="sym">{symbol}</span>
            </span>
          </h1>
        </div>
        <div className="actions" style={{ display: 'flex', gap: 8 }}>
          <button
            type="button"
            className="btn btn-primary"
            onClick={() => navigate(`/trade?symbol=${encodeURIComponent(symbol)}`)}
            disabled={!user?.email_verified || !symbolValid}
            title={user?.email_verified ? undefined : 'Verify your email to trade'}
          >
            Trade
          </button>
          <button
            type="button"
            className="btn btn-secondary"
            onClick={handleAddToWatchlist}
            disabled={watchAdding || !symbolValid}
          >
            {watchAdding ? 'Adding…' : '+ Watchlist'}
          </button>
        </div>
      </header>

      {!symbolValid && symbol && (
        <div className="alert alert-error">
          “{symbol}” isn’t a valid symbol. Use 1–10 letters, e.g. AAPL or BRK.B.
        </div>
      )}
      {quoteError && <div className="alert alert-error">{quoteError}</div>}
      {watchMessage && (
        <div className={`alert ${watchMessage.kind === 'err' ? 'alert-error' : 'alert-success'}`}>
          {watchMessage.text}
        </div>
      )}

      <section className="portfolio-hero">
        <div>
          <div className="label">Last close</div>
          <div className="display-num">
            <span className="currency">$</span>
            <span className="num">
              {quoteLoading ? '—' : quote ? formatMoney(quote.price) : '—'}
            </span>
          </div>
          {rangeStats && (
            <div className={`change-line ${trend}`}>
              <span>{formatSignedMoney(rangeStats.change)}</span>
              <span>({formatPercent(rangeStats.changePct)})</span>
              <span className="dot" />
              <span className="since">over {range.label}</span>
            </div>
          )}
        </div>
        <div className="meta">
          {quote?.date && <span className="accent-when">{quote.date}</span>}
        </div>
      </section>

      <section className="panel">
        <div className="panel-head">
          <h3>Price history</h3>
          <div className="tabs" role="tablist">
            {ranges.map((r) => (
              <button
                key={r.key}
                type="button"
                role="tab"
                aria-selected={range.key === r.key}
                className={`tab${range.key === r.key ? ' active' : ''}`}
                onClick={() => setRange(r)}
              >
                {r.label}
              </button>
            ))}
          </div>
        </div>

        <div style={{ padding: '12px 18px 18px' }}>
          {seriesLoading ? (
            <div className="empty-state">
              <span>Loading chart…</span>
            </div>
          ) : seriesError ? (
            <div className="empty-state">
              <span className="empty-title">{seriesError}</span>
            </div>
          ) : series && series.points.length >= 2 ? (
            <PriceChart points={series.points} />
          ) : (
            <div className="empty-state">
              <span className="empty-title">Not enough data to chart</span>
            </div>
          )}
        </div>

        {rangeStats && (
          <div
            className="mono"
            style={{
              display: 'grid',
              gridTemplateColumns: 'repeat(auto-fit, minmax(140px, 1fr))',
              gap: 12,
              padding: '12px 18px 18px',
              borderTop: '1px solid var(--hairline)',
              fontSize: 12,
            }}
          >
            <Stat label={`${range.label} high`} value={`$${formatMoney(rangeStats.high)}`} />
            <Stat label={`${range.label} low`} value={`$${formatMoney(rangeStats.low)}`} />
            <Stat label="Range start" value={`$${formatMoney(rangeStats.first)}`} />
            <Stat label="Range end" value={`$${formatMoney(rangeStats.last)}`} />
          </div>
        )}
      </section>
    </div>
  );
};

const Stat: React.FC<{ label: string; value: string }> = ({ label, value }) => (
  <div>
    <div style={{ color: 'var(--ink-muted)', fontSize: 11, textTransform: 'uppercase', letterSpacing: '0.06em' }}>
      {label}
    </div>
    <div style={{ marginTop: 2 }}>{value}</div>
  </div>
);

export default Stock;
