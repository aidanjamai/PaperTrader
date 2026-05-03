import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { apiRequest } from '../../services/api';
import { Trade, TradeHistoryResponse } from '../../types';
import { formatInteger, formatMoney } from '../primitives/format';

const PAGE_SIZE = 50;

type ActionFilter = 'ALL' | 'BUY' | 'SELL';

const formatDate = (iso: string): string => {
  const d = new Date(iso);
  if (isNaN(d.getTime())) return iso;
  const date = d.toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
  });
  const time = d.toLocaleTimeString('en-US', {
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  });
  return `${date} · ${time}`;
};

const History: React.FC = () => {
  const [trades, setTrades] = useState<Trade[]>([]);
  const [total, setTotal] = useState<number>(0);
  const [offset, setOffset] = useState<number>(0);
  const [symbolFilter, setSymbolFilter] = useState<string>('');
  const [symbolQuery, setSymbolQuery] = useState<string>('');
  const [actionFilter, setActionFilter] = useState<ActionFilter>('ALL');
  const [loading, setLoading] = useState<boolean>(false);
  const [error, setError] = useState<string | null>(null);

  const fetchHistory = useCallback(async () => {
    setLoading(true);
    setError(null);

    const params = new URLSearchParams();
    params.set('limit', String(PAGE_SIZE));
    params.set('offset', String(offset));
    if (symbolQuery) params.set('symbol', symbolQuery.toUpperCase());
    if (actionFilter !== 'ALL') params.set('action', actionFilter);

    try {
      const res = await apiRequest<TradeHistoryResponse>(
        `/investments/history?${params.toString()}`
      );
      if (!res.ok) {
        if (res.status === 401) {
          setError('Authentication required. Please log in again.');
        } else if (res.status === 400) {
          const txt = await res.text().catch(() => '');
          setError(txt || 'Invalid filter.');
        } else {
          setError(`Failed to load trade history (${res.status}).`);
        }
        setLoading(false);
        return;
      }
      const data = (await res.json()) as TradeHistoryResponse;
      setTrades(Array.isArray(data.trades) ? data.trades : []);
      setTotal(typeof data.total === 'number' ? data.total : 0);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load trade history.');
    } finally {
      setLoading(false);
    }
  }, [offset, symbolQuery, actionFilter]);

  useEffect(() => {
    fetchHistory();
  }, [fetchHistory]);

  const showingFrom = total === 0 ? 0 : offset + 1;
  const showingTo = Math.min(offset + PAGE_SIZE, total);
  const hasPrev = offset > 0;
  const hasNext = offset + PAGE_SIZE < total;

  const applySymbolFilter = () => {
    setOffset(0);
    setSymbolQuery(symbolFilter.trim());
  };

  const clearFilters = () => {
    setSymbolFilter('');
    setSymbolQuery('');
    setActionFilter('ALL');
    setOffset(0);
  };

  const handleAction = (next: ActionFilter) => {
    setOffset(0);
    setActionFilter(next);
  };

  const summary = useMemo(() => {
    if (total === 0) return 'No trades match these filters';
    return `Showing ${formatInteger(showingFrom)}–${formatInteger(showingTo)} of ${formatInteger(total)}`;
  }, [total, showingFrom, showingTo]);

  const filtered = symbolQuery || actionFilter !== 'ALL';

  return (
    <div className="container">
      <header className="page-header">
        <div>
          <div className="eyebrow" style={{ marginBottom: 6 }}>
            Ledger · all activity
          </div>
          <h1>Trade history</h1>
        </div>
        <div className="meta">
          <span className="accent-when">{summary}</span>
        </div>
      </header>

      <div
        style={{
          display: 'flex',
          gap: 12,
          flexWrap: 'wrap',
          alignItems: 'center',
          marginBottom: 16,
        }}
      >
        <input
          type="text"
          className="input mono"
          placeholder="Filter symbol (AAPL)"
          value={symbolFilter}
          onChange={(e) => setSymbolFilter(e.target.value.toUpperCase())}
          onKeyDown={(e) => {
            if (e.key === 'Enter') applySymbolFilter();
          }}
          style={{ width: 200 }}
        />
        <button type="button" className="btn btn-secondary btn-sm" onClick={applySymbolFilter}>
          Apply
        </button>

        <div className="tabs" role="tablist" style={{ marginLeft: 8 }}>
          {(['ALL', 'BUY', 'SELL'] as const).map((opt) => (
            <button
              key={opt}
              type="button"
              role="tab"
              aria-selected={actionFilter === opt}
              className={`tab${actionFilter === opt ? ' active' : ''}`}
              onClick={() => handleAction(opt)}
            >
              {opt === 'ALL' ? 'All' : opt === 'BUY' ? 'Buy' : 'Sell'}
            </button>
          ))}
        </div>

        {filtered && (
          <button
            type="button"
            className="btn btn-ghost btn-sm"
            onClick={clearFilters}
            style={{ marginLeft: 8 }}
          >
            Clear filters
          </button>
        )}
      </div>

      {error && <div className="alert alert-error">{error}</div>}

      <div className="panel">
        {loading ? (
          <div className="empty-state">Loading trades…</div>
        ) : trades.length === 0 ? (
          <div className="empty-state">
            <span className="empty-title">
              {filtered ? 'No trades match these filters' : 'No trades yet'}
            </span>
            {!filtered && (
              <Link to="/trade" className="btn btn-primary">
                Place your first order
              </Link>
            )}
          </div>
        ) : (
          <table className="holdings">
            <thead>
              <tr>
                <th>Date</th>
                <th>Symbol</th>
                <th>Side</th>
                <th className="num">Qty</th>
                <th className="num">Price</th>
                <th className="num">Total</th>
              </tr>
            </thead>
            <tbody>
              {trades.map((t) => {
                const isBuy = t.action === 'BUY';
                return (
                  <tr key={t.id}>
                    <td className="mono" style={{ color: 'var(--ink-muted)' }} title={t.executed_at}>
                      {formatDate(t.executed_at)}
                    </td>
                    <td>
                      <div className="ticker">
                        <span className="mark" aria-hidden="true">
                          {t.symbol.charAt(0)}
                        </span>
                        <span className="sym">{t.symbol}</span>
                      </div>
                    </td>
                    <td>
                      <span className={`pill ${isBuy ? 'pill-gain' : 'pill-loss'}`}>
                        {t.action}
                      </span>
                    </td>
                    <td className="num">{formatInteger(t.quantity)}</td>
                    <td className="num">${formatMoney(t.price)}</td>
                    <td className="num">${formatMoney(t.total)}</td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        )}
      </div>

      <div
        style={{
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
          marginTop: 16,
          gap: 12,
          flexWrap: 'wrap',
        }}
      >
        <span className="mono" style={{ color: 'var(--ink-muted)', fontSize: 12 }}>
          {summary}
        </span>
        <div style={{ display: 'flex', gap: 8 }}>
          <button
            type="button"
            className="btn btn-secondary btn-sm"
            onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}
            disabled={!hasPrev || loading}
          >
            ← Prev
          </button>
          <button
            type="button"
            className="btn btn-secondary btn-sm"
            onClick={() => setOffset(offset + PAGE_SIZE)}
            disabled={!hasNext || loading}
          >
            Next →
          </button>
        </div>
      </div>
    </div>
  );
};

export default History;
