import React, { useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { UserStock } from '../../types';
import { usePortfolio } from '../../hooks/usePortfolio';
import { useAuth } from '../../hooks/useAuth';

const Dashboard: React.FC = () => {
  const { stocks, loading, fetchPortfolio } = usePortfolio();
  const { user } = useAuth();
  const navigate = useNavigate();

  useEffect(() => {
    fetchPortfolio();
  }, [fetchPortfolio]);


  const portfolioValue = stocks.reduce((total: number, stock: UserStock) => {
    return total + (stock.quantity * (stock.current_stock_price || 0));
  }, 0);

  
  const calculateProfitLoss = (stock: UserStock): number => {
    return stock.quantity * stock.current_stock_price - stock.quantity * stock.avg_price;
  };

  const getValueColor = (value: number): string | undefined => {
    if (value > 0) return '#10b981';
    if (value < 0) return '#ef4444';
    return undefined;
  };

  const totalValue = portfolioValue + (user?.balance || 0);

  const handleStartTrading = () => {
    navigate('/trade');
  };

  return (
    <div className="dashboard">
      <h1>Welcome to Your Dashboard</h1>
      
      {/* <div className="user-info">
        <h3>Account Information</h3>
        <p><strong>Email:</strong> {user?.email}</p>
        <p><strong>Member Since:</strong> {new Date(user?.created_at).toLocaleDateString()}</p>
        <p><strong>Account Balance:</strong> ${user?.balance?.toFixed(2) || '0.00'}</p>
      </div> */}

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(250px, 1fr))', gap: '20px' }}>
        <div style={{ background: '#f8f9fa', padding: '20px', borderRadius: '8px' }}>
          <h3 style={{ color: '#667eea', marginBottom: '12px' }}>Portfolio Value</h3>
          <p style={{ fontSize: '24px', fontWeight: 'bold', color: '#333' }}>${portfolioValue.toFixed(2)}</p>
          <p style={{ color: '#666', fontSize: '14px' }}>{stocks.length} positions</p>
        </div>

        <div style={{ background: '#f8f9fa', padding: '20px', borderRadius: '8px' }}>
          <h3 style={{ color: '#667eea', marginBottom: '12px' }}>Available Cash</h3>
          <p style={{ fontSize: '24px', fontWeight: 'bold', color: '#333' }}>${user?.balance?.toFixed(2) || '0.00'}</p>
          <p style={{ color: '#666', fontSize: '14px' }}>Buying power</p>
        </div>

        <div style={{ background: '#f8f9fa', padding: '20px', borderRadius: '8px' }}>
          <h3 style={{ color: '#667eea', marginBottom: '12px' }}>Total Net Worth</h3>
          <p style={{ fontSize: '24px', fontWeight: 'bold', color: '#333' }}>${totalValue.toFixed(2)}</p>
          <p style={{ color: '#666', fontSize: '14px' }}>Cash + Investments</p>
        </div>
      </div>

      <div style={{ marginTop: '32px' }}>
        <h3 style={{ color: '#333', marginBottom: '16px' }}>Your Holdings</h3>
        {loading ? (
          <p>Loading portfolio...</p>
        ) : stocks.length > 0 ? (
          <div style={{ overflowX: 'auto' }}>
            <table className="table" style={{ width: '100%', borderCollapse: 'collapse', marginTop: '10px' }}>
              <thead>
                <tr style={{ background: '#f1f1f1', textAlign: 'left' }}>
                  <th style={{ padding: '12px' }}>Symbol</th>
                  <th style={{ padding: '12px' }}>Quantity</th>
                  <th style={{ padding: '12px' }}>Avg Price</th>
                  <th style={{ padding: '12px' }}>Current Price</th>
                  <th style={{ padding: '12px' }}>Total Value</th>
                  <th style={{ padding: '12px' }}>Profit/Loss</th>
                </tr>
              </thead>
              <tbody>
                {stocks.map((stock: UserStock) => {
                  const profitLoss = calculateProfitLoss(stock);
                  const profitLossColor = getValueColor(profitLoss);
                  return (
                    <tr key={stock.symbol} style={{ borderBottom: '1px solid #eee' }}>
                      <td style={{ padding: '12px' }}><strong>{stock.symbol}</strong></td>
                      <td style={{ padding: '12px' }}>{stock.quantity}</td>
                      <td style={{ padding: '12px' }}>${stock.avg_price?.toFixed(2)}</td>
                      <td style={{ padding: '12px' }}>${stock.current_stock_price?.toFixed(2)}</td>
                      <td style={{ padding: '12px' }}>${(stock.quantity * stock.current_stock_price).toFixed(2)}</td>
                      <td style={{ padding: '12px', color: profitLossColor }}>${profitLoss.toFixed(2)}</td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        ) : (
          <div style={{ textAlign: 'center', padding: '40px', background: '#f9f9f9', borderRadius: '8px' }}>
            <p style={{ color: '#666', marginBottom: '16px' }}>No positions yet</p>
            <button className="btn btn-primary" onClick={handleStartTrading}>
              Start Trading
            </button>
          </div>
        )}
      </div>
    </div>
  );
};

export default Dashboard;

