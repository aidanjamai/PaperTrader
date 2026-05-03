import React, { useEffect, useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { UserStock } from '../../types';
import { usePortfolio } from '../../hooks/usePortfolio';
import { useAuth } from '../../hooks/useAuth';
import {
  formatInteger,
  formatMoney,
  formatPercent,
  formatSignedMoney,
  formatTimestamp,
} from '../primitives/format';
import WatchlistCard from './WatchlistCard';

type FilterKey = 'all' | 'stocks' | 'etfs';

const ETF_SYMBOLS = new Set([
  'VOO', 'SPY', 'QQQ', 'VTI', 'IVV', 'VEA', 'VWO', 'AGG', 'BND', 'IWM',
  'GLD', 'SLV', 'DIA', 'EFA', 'IEMG', 'VXUS', 'VNQ', 'SCHD', 'XLK', 'XLF',
]);

const isEtf = (symbol: string): boolean => ETF_SYMBOLS.has(symbol.toUpperCase());

const Dashboard: React.FC = () => {
  const { stocks, loading, error, fetchPortfolio } = usePortfolio();
  const { user } = useAuth();
  const navigate = useNavigate();
  const [filter, setFilter] = useState<FilterKey>('all');

  useEffect(() => {
    fetchPortfolio();
  }, [fetchPortfolio]);

  const enriched = useMemo(
    () =>
      stocks.map((s) => {
        const value = s.quantity * s.current_stock_price;
        const cost = s.quantity * s.avg_price;
        const pnl = value - cost;
        const pnlPct = cost > 0 ? (pnl / cost) * 100 : 0;
        return { ...s, value, cost, pnl, pnlPct };
      }),
    [stocks]
  );

  const counts = useMemo(() => {
    let etfs = 0;
    enriched.forEach((s) => {
      if (isEtf(s.symbol)) etfs += 1;
    });
    return {
      all: enriched.length,
      stocks: enriched.length - etfs,
      etfs,
    };
  }, [enriched]);

  const filtered = useMemo(() => {
    if (filter === 'all') return enriched;
    if (filter === 'etfs') return enriched.filter((s) => isEtf(s.symbol));
    return enriched.filter((s) => !isEtf(s.symbol));
  }, [enriched, filter]);

  const invested = enriched.reduce((sum, s) => sum + s.value, 0);
  const totalCost = enriched.reduce((sum, s) => sum + s.cost, 0);
  const cash = user?.balance ?? 0;
  const totalValue = invested + cash;
  const sessionPnl = invested - totalCost;
  const sessionPnlPct = totalCost > 0 ? (sessionPnl / totalCost) * 100 : 0;
  const upCount = enriched.filter((s) => s.pnl > 0).length;
  const downCount = enriched.filter((s) => s.pnl < 0).length;

  const trend: 'gain' | 'loss' | 'flat' =
    sessionPnl > 0 ? 'gain' : sessionPnl < 0 ? 'loss' : 'flat';

  const cashPct = totalValue > 0 ? (cash / totalValue) * 100 : 0;

  return (
    <div className="container">
      <header className="page-header">
        <div>
          <div className="eyebrow" style={{ marginBottom: 6 }}>
            Portfolio · USD
          </div>
          <h1>Dashboard</h1>
        </div>
        <div className="meta">
          As of <span className="accent-when">{formatTimestamp()}</span>
        </div>
      </header>

      {error && <div className="alert alert-error">{error}</div>}

      <section className="portfolio-hero">
        <div>
          <div className="label">Total portfolio value</div>
          <div className="display-num">
            <span className="currency">$</span>
            <span className="num">{formatMoney(totalValue)}</span>
          </div>
          <div className={`change-line ${trend}`}>
            <span>{formatSignedMoney(sessionPnl)}</span>
            <span>({formatPercent(sessionPnlPct)})</span>
            <span className="dot" />
            <span className="since">unrealised · vs cost basis</span>
          </div>
        </div>
        <div className="actions">
          <button
            type="button"
            className="btn btn-primary btn-lg"
            onClick={() => navigate('/trade')}
            disabled={!user?.email_verified}
            title={user?.email_verified ? undefined : 'Verify your email to trade'}
          >
            Trade
            <svg className="ico" viewBox="0 0 16 16" aria-hidden="true">
              <path d="M3 8h10" />
              <path d="M9 4l4 4-4 4" />
            </svg>
          </button>
          <button
            type="button"
            className="btn btn-secondary"
            onClick={() => navigate('/markets')}
          >
            Browse markets
          </button>
        </div>
      </section>

      <section className="stat-row">
        <div className="stat">
          <div className="k">Cash available</div>
          <div className="v">${formatMoney(cash)}</div>
          <div className="sub">
            {totalValue > 0 ? `${cashPct.toFixed(1)}% of portfolio` : 'Buying power'}
          </div>
        </div>
        <div className="stat">
          <div className="k">Invested</div>
          <div className="v">${formatMoney(invested)}</div>
          <div className="sub">
            across {formatInteger(enriched.length)}{' '}
            {enriched.length === 1 ? 'position' : 'positions'}
          </div>
        </div>
        <div className="stat">
          <div className="k">Unrealised P/L</div>
          <div className={`v ${trend === 'loss' ? 'loss-text' : trend === 'gain' ? 'gain-text' : ''}`}>
            {formatSignedMoney(sessionPnl)}
          </div>
          <div className={`sub ${trend === 'gain' ? 'gain' : trend === 'loss' ? 'loss' : ''}`}>
            {formatPercent(sessionPnlPct)} · {upCount} up · {downCount} down
          </div>
        </div>
      </section>

      <section className="panel">
        <div className="panel-head">
          <h3>Holdings</h3>
          <div className="tabs" role="tablist">
            <button
              type="button"
              role="tab"
              aria-selected={filter === 'all'}
              className={`tab${filter === 'all' ? ' active' : ''}`}
              onClick={() => setFilter('all')}
            >
              All · {counts.all}
            </button>
            <button
              type="button"
              role="tab"
              aria-selected={filter === 'stocks'}
              className={`tab${filter === 'stocks' ? ' active' : ''}`}
              onClick={() => setFilter('stocks')}
            >
              Stocks · {counts.stocks}
            </button>
            <button
              type="button"
              role="tab"
              aria-selected={filter === 'etfs'}
              className={`tab${filter === 'etfs' ? ' active' : ''}`}
              onClick={() => setFilter('etfs')}
            >
              ETFs · {counts.etfs}
            </button>
          </div>
        </div>

        {loading ? (
          <div className="empty-state">
            <span>Loading portfolio…</span>
          </div>
        ) : filtered.length === 0 ? (
          <div className="empty-state">
            <span className="empty-title">
              {enriched.length === 0 ? 'No positions yet' : 'No holdings in this view'}
            </span>
            {enriched.length === 0 && (
              <button
                type="button"
                className="btn btn-primary"
                onClick={() => navigate('/trade')}
                disabled={!user?.email_verified}
              >
                Start trading
              </button>
            )}
          </div>
        ) : (
          <table className="holdings">
            <thead>
              <tr>
                <th>Symbol</th>
                <th className="num">Qty</th>
                <th className="num">Avg price</th>
                <th className="num">Current</th>
                <th className="num">Value</th>
                <th className="num">P/L</th>
                <th className="num">% P/L</th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((s: UserStock & { value: number; pnl: number; pnlPct: number }) => {
                const tone = s.pnl > 0 ? 'gain-text' : s.pnl < 0 ? 'loss-text' : '';
                return (
                  <tr key={s.symbol}>
                    <td>
                      <div className="ticker">
                        <span className="mark" aria-hidden="true">
                          {s.symbol.charAt(0)}
                        </span>
                        <span className="sym">{s.symbol}</span>
                        <span className="name">
                          {isEtf(s.symbol) ? 'ETF' : ''}
                        </span>
                      </div>
                    </td>
                    <td className="num">{formatInteger(s.quantity)}</td>
                    <td className="num">{formatMoney(s.avg_price)}</td>
                    <td className="num">{formatMoney(s.current_stock_price)}</td>
                    <td className="num">{formatMoney(s.value)}</td>
                    <td className={`num ${tone}`}>{formatSignedMoney(s.pnl)}</td>
                    <td className={`num ${tone}`}>{formatPercent(s.pnlPct)}</td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        )}
      </section>

      <WatchlistCard />
    </div>
  );
};

export default Dashboard;
