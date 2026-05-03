import React, { useState, ChangeEvent, useEffect } from 'react';
import { usePortfolio } from '../../hooks/usePortfolio';
import { UserStock } from '../../types';
import { formatMoney, formatPercent, formatSignedMoney } from '../primitives/format';
import ToolsTabs from './ToolsTabs';

interface StockInput {
  id: number;
  symbol: string;
  currentPrice: string;
  sharesOwned: string;
  futurePrice: string;
}

interface StockCalculation {
  currentValue: number;
  futureValue: number;
  gainLoss: number;
  gainLossPercent: number;
}

interface PortfolioTotals {
  totalCurrentValue: number;
  totalFutureValue: number;
  totalGainLoss: number;
  totalGainLossPercent: number;
}

const Calculator: React.FC = () => {
  const {
    stocks: portfolioStocks,
    loading: portfolioLoading,
    error: portfolioError,
    fetchPortfolio,
  } = usePortfolio();
  const [stocks, setStocks] = useState<StockInput[]>([
    { id: 1, symbol: '', currentPrice: '', sharesOwned: '', futurePrice: '' },
  ]);
  const [nextId, setNextId] = useState<number>(2);
  const [portfolioLoaded, setPortfolioLoaded] = useState<boolean>(false);

  const addStock = () => {
    setStocks([
      ...stocks,
      { id: nextId, symbol: '', currentPrice: '', sharesOwned: '', futurePrice: '' },
    ]);
    setNextId((prev) => prev + 1);
  };

  const removeStock = (id: number) => {
    if (stocks.length > 1) setStocks(stocks.filter((s) => s.id !== id));
  };

  const updateStock = (id: number, field: keyof StockInput, value: string) => {
    setStocks(stocks.map((s) => (s.id === id ? { ...s, [field]: value } : s)));
  };

  const calculateStockGain = (stock: StockInput): StockCalculation | null => {
    const current = parseFloat(stock.currentPrice) || 0;
    const future = parseFloat(stock.futurePrice) || 0;
    const shares = parseInt(stock.sharesOwned, 10) || 0;
    if (current === 0 || shares === 0) return null;
    const currentValue = current * shares;
    const futureValue = future * shares;
    const gainLoss = futureValue - currentValue;
    const gainLossPercent = (gainLoss / currentValue) * 100;
    return { currentValue, futureValue, gainLoss, gainLossPercent };
  };

  const calculateTotals = (): PortfolioTotals => {
    let totalCurrentValue = 0;
    let totalFutureValue = 0;
    let totalGainLoss = 0;
    stocks.forEach((s) => {
      const c = calculateStockGain(s);
      if (c) {
        totalCurrentValue += c.currentValue;
        totalFutureValue += c.futureValue;
        totalGainLoss += c.gainLoss;
      }
    });
    const totalGainLossPercent =
      totalCurrentValue > 0 ? (totalGainLoss / totalCurrentValue) * 100 : 0;
    return { totalCurrentValue, totalFutureValue, totalGainLoss, totalGainLossPercent };
  };

  const convertPortfolioToCalculator = (ps: UserStock[]): StockInput[] =>
    ps.map((s, index) => ({
      id: index + 1,
      symbol: s.symbol,
      currentPrice:
        s.current_stock_price > 0
          ? s.current_stock_price.toFixed(2)
          : s.avg_price.toFixed(2),
      sharesOwned: s.quantity.toString(),
      futurePrice: '',
    }));

  const loadFromPortfolio = async () => {
    setPortfolioLoaded(false);
    await fetchPortfolio();
  };

  useEffect(() => {
    if (portfolioStocks.length > 0 && !portfolioLoading && !portfolioLoaded) {
      const converted = convertPortfolioToCalculator(portfolioStocks);
      if (converted.length > 0) {
        setStocks(converted);
        setNextId(converted.length + 1);
        setPortfolioLoaded(true);
      }
    }
  }, [portfolioStocks, portfolioLoading, portfolioLoaded]);

  const totals = calculateTotals();
  const totalsTone: 'gain' | 'loss' | 'flat' =
    totals.totalGainLoss > 0 ? 'gain' : totals.totalGainLoss < 0 ? 'loss' : 'flat';

  return (
    <div className="container" style={{ paddingTop: 24 }}>
      <header className="page-header">
        <div>
          <div className="eyebrow" style={{ marginBottom: 6 }}>
            Tools · scenario
          </div>
          <h1>Portfolio calculator</h1>
        </div>
        <div className="meta">Project potential gains across many positions</div>
      </header>

      <ToolsTabs />

      <div
        className="panel"
        style={{ padding: '18px 22px', marginBottom: 16, display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 16, flexWrap: 'wrap' }}
      >
        <div>
          <div className="eyebrow" style={{ marginBottom: 4 }}>
            Import
          </div>
          <div style={{ fontSize: 14 }}>
            Pre-fill from your live portfolio.
          </div>
        </div>
        <button
          type="button"
          onClick={loadFromPortfolio}
          className="btn btn-secondary"
          disabled={portfolioLoading}
        >
          {portfolioLoading ? 'Loading…' : 'Load from portfolio'}
        </button>
      </div>

      {portfolioError && <div className="alert alert-error">{portfolioError}</div>}
      {portfolioLoaded && portfolioStocks.length > 0 && (
        <div className="alert alert-success">
          Loaded {portfolioStocks.length} position{portfolioStocks.length !== 1 ? 's' : ''} from
          your portfolio.
        </div>
      )}

      <div style={{ display: 'grid', gap: 16, marginBottom: 24 }}>
        {stocks.map((stock, index) => {
          const calc = calculateStockGain(stock);
          const tone =
            calc && calc.gainLoss > 0 ? 'gain' : calc && calc.gainLoss < 0 ? 'loss' : null;
          return (
            <div
              key={stock.id}
              className="panel"
              style={{ padding: '18px 22px' }}
            >
              <div
                style={{
                  display: 'flex',
                  justifyContent: 'space-between',
                  alignItems: 'center',
                  marginBottom: 14,
                }}
              >
                <div className="eyebrow">Position #{index + 1}</div>
                {stocks.length > 1 && (
                  <button
                    type="button"
                    onClick={() => removeStock(stock.id)}
                    className="btn btn-ghost btn-sm"
                  >
                    Remove
                  </button>
                )}
              </div>

              <div
                style={{
                  display: 'grid',
                  gridTemplateColumns: 'repeat(auto-fit, minmax(160px, 1fr))',
                  gap: 14,
                }}
              >
                <div className="form-group" style={{ marginBottom: 0 }}>
                  <label htmlFor={`symbol-${stock.id}`} className="form-label">
                    Symbol
                  </label>
                  <input
                    type="text"
                    id={`symbol-${stock.id}`}
                    className="input mono"
                    value={stock.symbol}
                    onChange={(e: ChangeEvent<HTMLInputElement>) =>
                      updateStock(stock.id, 'symbol', e.target.value.toUpperCase())
                    }
                    placeholder="AAPL"
                    maxLength={10}
                  />
                </div>

                <div className="form-group" style={{ marginBottom: 0 }}>
                  <label htmlFor={`current-${stock.id}`} className="form-label">
                    Current price
                  </label>
                  <input
                    type="number"
                    id={`current-${stock.id}`}
                    className="input mono"
                    value={stock.currentPrice}
                    onChange={(e: ChangeEvent<HTMLInputElement>) =>
                      updateStock(stock.id, 'currentPrice', e.target.value)
                    }
                    placeholder="0.00"
                    min="0"
                    step="0.01"
                  />
                </div>

                <div className="form-group" style={{ marginBottom: 0 }}>
                  <label htmlFor={`shares-${stock.id}`} className="form-label">
                    Shares
                  </label>
                  <input
                    type="number"
                    id={`shares-${stock.id}`}
                    className="input mono"
                    value={stock.sharesOwned}
                    onChange={(e: ChangeEvent<HTMLInputElement>) =>
                      updateStock(stock.id, 'sharesOwned', e.target.value)
                    }
                    placeholder="0"
                    min="0"
                    step="1"
                  />
                </div>

                <div className="form-group" style={{ marginBottom: 0 }}>
                  <label htmlFor={`future-${stock.id}`} className="form-label">
                    Future price
                  </label>
                  <input
                    type="number"
                    id={`future-${stock.id}`}
                    className="input mono"
                    value={stock.futurePrice}
                    onChange={(e: ChangeEvent<HTMLInputElement>) =>
                      updateStock(stock.id, 'futurePrice', e.target.value)
                    }
                    placeholder="0.00"
                    min="0"
                    step="0.01"
                  />
                </div>
              </div>

              {calc && (
                <div
                  className="mono"
                  style={{
                    marginTop: 14,
                    paddingTop: 14,
                    borderTop: '1px solid var(--hairline)',
                    display: 'grid',
                    gridTemplateColumns: 'repeat(auto-fit, minmax(140px, 1fr))',
                    gap: 12,
                    fontSize: 13,
                  }}
                >
                  <Cell label="Current" value={`$${formatMoney(calc.currentValue)}`} />
                  <Cell label="Future" value={`$${formatMoney(calc.futureValue)}`} />
                  <Cell
                    label="Δ"
                    value={formatSignedMoney(calc.gainLoss)}
                    tone={tone}
                  />
                  <Cell
                    label="Δ %"
                    value={formatPercent(calc.gainLossPercent)}
                    tone={tone}
                  />
                </div>
              )}
            </div>
          );
        })}
      </div>

      <div style={{ marginBottom: 24 }}>
        <button type="button" onClick={addStock} className="btn btn-secondary">
          + Add position
        </button>
      </div>

      <div className="portfolio-hero" style={{ marginBottom: 0 }}>
        <div>
          <div className="label">Projected total value</div>
          <div className="display-num">
            <span className="currency">$</span>
            <span className="num">{formatMoney(totals.totalFutureValue)}</span>
          </div>
          <div className={`change-line ${totalsTone}`}>
            <span>{formatSignedMoney(totals.totalGainLoss)}</span>
            <span>({formatPercent(totals.totalGainLossPercent)})</span>
            <span className="dot" />
            <span className="since">
              from ${formatMoney(totals.totalCurrentValue)} today
            </span>
          </div>
        </div>
      </div>
    </div>
  );
};

const Cell: React.FC<{ label: string; value: string; tone?: 'gain' | 'loss' | null }> = ({
  label,
  value,
  tone,
}) => (
  <div>
    <div style={{ color: 'var(--ink-muted)', fontSize: 11 }}>{label}</div>
    <div
      style={{
        color: tone === 'gain' ? 'var(--gain)' : tone === 'loss' ? 'var(--loss)' : 'var(--ink)',
        fontWeight: 500,
        marginTop: 2,
      }}
    >
      {value}
    </div>
  </div>
);

export default Calculator;
