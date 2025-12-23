import React, { useState, ChangeEvent } from 'react';

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
  const [stocks, setStocks] = useState<StockInput[]>([
    { id: 1, symbol: '', currentPrice: '', sharesOwned: '', futurePrice: '' }
  ]);
  const [nextId, setNextId] = useState<number>(2);

  const addStock = () => {
    setStocks([
      ...stocks,
      { id: nextId, symbol: '', currentPrice: '', sharesOwned: '', futurePrice: '' }
    ]);
    setNextId(prev => prev + 1);
  };

  const removeStock = (id: number) => {
    if (stocks.length > 1) {
      setStocks(stocks.filter(stock => stock.id !== id));
    }
  };

  const updateStock = (id: number, field: keyof StockInput, value: string) => {
    setStocks(stocks.map(stock =>
      stock.id === id ? { ...stock, [field]: value } : stock
    ));
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
    
    return {
      currentValue,
      futureValue,
      gainLoss,
      gainLossPercent
    };
  };

  const calculateTotals = (): PortfolioTotals => {
    let totalCurrentValue = 0;
    let totalFutureValue = 0;
    let totalGainLoss = 0;
    
    stocks.forEach(stock => {
      const calc = calculateStockGain(stock);
      if (calc) {
        totalCurrentValue += calc.currentValue;
        totalFutureValue += calc.futureValue;
        totalGainLoss += calc.gainLoss;
      }
    });
    
    const totalGainLossPercent = totalCurrentValue > 0 ? (totalGainLoss / totalCurrentValue) * 100 : 0;
    
    return {
      totalCurrentValue,
      totalFutureValue,
      totalGainLoss,
      totalGainLossPercent
    };
  };

  const totals = calculateTotals();

  return (
    <div style={{ marginTop: '60px' }}>
      <div className="card calculator-card">
        <h2>Stock Portfolio Calculator</h2>
        <p style={{ color: '#666', marginBottom: '20px' }}>
          Calculate potential gains/losses by entering your current holdings and projected future prices.
        </p>

        {stocks.map((stock, index) => (
          <div key={stock.id} style={{
            border: '1px solid #ddd',
            borderRadius: '8px',
            padding: '20px',
            marginBottom: '20px',
            backgroundColor: '#f9f9f9'
          }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '15px' }}>
              <h4 style={{ margin: 0, color: '#333' }}>Stock #{index + 1}</h4>
              {stocks.length > 1 && (
                <button
                  type="button"
                  onClick={() => removeStock(stock.id)}
                  className="btn btn-secondary"
                  style={{ padding: '5px 10px', fontSize: '12px' }}
                >
                  Remove
                </button>
              )}
            </div>

            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))', gap: '15px' }}>
              <div className="form-group">
                <label htmlFor={`symbol-${stock.id}`}>Stock Symbol</label>
                <input
                  type="text"
                  id={`symbol-${stock.id}`}
                  className="form-control"
                  value={stock.symbol}
                  onChange={(e: ChangeEvent<HTMLInputElement>) => updateStock(stock.id, 'symbol', e.target.value.toUpperCase())}
                  placeholder="e.g., AAPL"
                  maxLength={10}
                />
              </div>

              <div className="form-group">
                <label htmlFor={`current-${stock.id}`}>Current Price ($)</label>
                <input
                  type="number"
                  id={`current-${stock.id}`}
                  className="form-control"
                  value={stock.currentPrice}
                  onChange={(e: ChangeEvent<HTMLInputElement>) => updateStock(stock.id, 'currentPrice', e.target.value)}
                  placeholder="0.00"
                  min="0"
                  step="0.01"
                />
              </div>

              <div className="form-group">
                <label htmlFor={`shares-${stock.id}`}>Shares Owned</label>
                <input
                  type="number"
                  id={`shares-${stock.id}`}
                  className="form-control"
                  value={stock.sharesOwned}
                  onChange={(e: ChangeEvent<HTMLInputElement>) => updateStock(stock.id, 'sharesOwned', e.target.value)}
                  placeholder="0"
                  min="0"
                  step="1"
                />
              </div>

              <div className="form-group">
                <label htmlFor={`future-${stock.id}`}>Future Price ($)</label>
                <input
                  type="number"
                  id={`future-${stock.id}`}
                  className="form-control"
                  value={stock.futurePrice}
                  onChange={(e: ChangeEvent<HTMLInputElement>) => updateStock(stock.id, 'futurePrice', e.target.value)}
                  placeholder="0.00"
                  min="0"
                  step="0.01"
                />
              </div>
            </div>

            {/* Individual Stock Calculation */}
            {(() => {
              const calc = calculateStockGain(stock);
              if (!calc) return null;
              
              return (
                <div style={{
                  marginTop: '15px',
                  padding: '10px',
                  backgroundColor: '#fff',
                  borderRadius: '4px',
                  border: '1px solid #e0e0e0'
                }}>
                  <h5 style={{ margin: '0 0 10px 0', color: '#333' }}>Calculation for {stock.symbol || 'Stock'}:</h5>
                  <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(150px, 1fr))', gap: '10px', fontSize: '14px' }}>
                    <div>
                      <strong>Current Value:</strong><br />
                      ${calc.currentValue.toFixed(2)}
                    </div>
                    <div>
                      <strong>Future Value:</strong><br />
                      ${calc.futureValue.toFixed(2)}
                    </div>
                    <div>
                      <strong>Gain/Loss:</strong><br />
                      <span style={{ color: calc.gainLoss >= 0 ? '#28a745' : '#dc3545' }}>
                        ${calc.gainLoss.toFixed(2)}
                      </span>
                    </div>
                    <div>
                      <strong>Gain/Loss %:</strong><br />
                      <span style={{ color: calc.gainLossPercent >= 0 ? '#28a745' : '#dc3545' }}>
                        {calc.gainLossPercent.toFixed(2)}%
                      </span>
                    </div>
                  </div>
                </div>
              );
            })()}
          </div>
        ))}

        <div style={{ textAlign: 'center', marginBottom: '20px' }}>
          <button
            type="button"
            onClick={addStock}
            className="btn btn-primary"
            style={{ marginRight: '10px' }}
          >
            Add Another Stock
          </button>
        </div>

        {/* Portfolio Totals */}
        <div style={{
          border: '2px solid #007bff',
          borderRadius: '8px',
          padding: '20px',
          backgroundColor: '#f8f9fa',
          marginTop: '20px'
        }}>
          <h3 style={{ margin: '0 0 15px 0', color: '#007bff', textAlign: 'center' }}>Portfolio Summary</h3>
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))', gap: '15px' }}>
            <div style={{ textAlign: 'center' }}>
              <h4 style={{ margin: '0 0 5px 0', color: '#666' }}>Current Total Value</h4>
              <div style={{ fontSize: '24px', fontWeight: 'bold', color: '#333' }}>
                ${totals.totalCurrentValue.toFixed(2)}
              </div>
            </div>
            <div style={{ textAlign: 'center' }}>
              <h4 style={{ margin: '0 0 5px 0', color: '#666' }}>Future Total Value</h4>
              <div style={{ fontSize: '24px', fontWeight: 'bold', color: '#333' }}>
                ${totals.totalFutureValue.toFixed(2)}
              </div>
            </div>
            <div style={{ textAlign: 'center' }}>
              <h4 style={{ margin: '0 0 5px 0', color: '#666' }}>Total Gain/Loss</h4>
              <div style={{
                fontSize: '24px',
                fontWeight: 'bold',
                color: totals.totalGainLoss >= 0 ? '#28a745' : '#dc3545'
              }}>
                ${totals.totalGainLoss.toFixed(2)}
              </div>
            </div>
            <div style={{ textAlign: 'center' }}>
              <h4 style={{ margin: '0 0 5px 0', color: '#666' }}>Total Gain/Loss %</h4>
              <div style={{
                fontSize: '24px',
                fontWeight: 'bold',
                color: totals.totalGainLossPercent >= 0 ? '#28a745' : '#dc3545'
              }}>
                {totals.totalGainLossPercent.toFixed(2)}%
              </div>
            </div>
          </div>
        </div>

        {/* Instructions */}
        <div style={{ marginTop: '20px', padding: '15px', backgroundColor: '#e9ecef', borderRadius: '4px' }}>
          <h5 style={{ margin: '0 0 10px 0' }}>How to use:</h5>
          <ul style={{ margin: 0, paddingLeft: '20px' }}>
            <li>Enter the stock symbol (e.g., AAPL, GOOGL)</li>
            <li>Input the current price per share</li>
            <li>Enter the number of shares you own</li>
            <li>Set your projected future price per share</li>
            <li>View individual calculations and portfolio totals below</li>
          </ul>
        </div>
      </div>
    </div>
  );
};

export default Calculator;

