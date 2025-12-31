import React, { useState, FormEvent, ChangeEvent } from 'react';
import { useNavigate } from 'react-router-dom';
import { apiRequest } from '../../services/api';
import { User, TradeAction, BuyStockRequest, SellStockRequest, UserStock } from '../../types';

interface TradeProps {
  user: User;
}

const Trade: React.FC<TradeProps> = ({ user }) => {
  const navigate = useNavigate();
  const [tradeType, setTradeType] = useState<TradeAction>('buy');
  const [symbol, setSymbol] = useState<string>('');
  const [quantity, setQuantity] = useState<string>('');
  const [loading, setLoading] = useState<boolean>(false);
  const [message, setMessage] = useState<string>('');
  const [error, setError] = useState<string>('');

  const handleSubmit = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    setError('');
    setMessage('');
    setLoading(true);

    // Validation
    if (!symbol.trim()) {
      setError('Please enter a stock symbol');
      setLoading(false);
      return;
    }

    const quantityNum = parseInt(quantity, 10);
    if (isNaN(quantityNum) || quantityNum <= 0) {
      setError('Quantity must be a positive whole number');
      setLoading(false);
      return;
    }

    try {
      const endpoint = tradeType === 'buy' ? '/investments/buy' : '/investments/sell';
      const requestBody: BuyStockRequest | SellStockRequest = {
        symbol: symbol.toUpperCase().trim(),
        quantity: quantityNum
      };
      
      const response = await apiRequest<UserStock>(endpoint, {
        method: 'POST',
        body: JSON.stringify(requestBody)
      });

      if (response.ok) {
        await response.json();
        setMessage(`${tradeType === 'buy' ? 'Bought' : 'Sold'} ${quantityNum} shares of ${symbol.toUpperCase()} successfully!`);
        setSymbol('');
        setQuantity('');
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

  const handleSymbolChange = (e: ChangeEvent<HTMLInputElement>) => {
    setSymbol(e.target.value.toUpperCase());
  };

  const handleQuantityChange = (e: ChangeEvent<HTMLInputElement>) => {
    setQuantity(e.target.value);
  };

  // Show verification message if email not verified
  if (!user.email_verified) {
    return (
      <div style={{ marginTop: '60px' }}>
        <div className="card">
          <h2>Verify Your Email</h2>
          <div style={{
            background: '#fff3cd',
            padding: '20px',
            borderRadius: '8px',
            border: '1px solid #ffc107',
            marginBottom: '20px'
          }}>
            <p style={{ color: '#856404', marginBottom: '12px', fontWeight: '600' }}>
              Email verification required
            </p>
            <p style={{ color: '#856404', marginBottom: '16px' }}>
              Please verify your email address before you can start trading. Check the banner at the top of the page for instructions, or check your inbox for the verification email.
            </p>
            <button 
              className="btn btn-primary"
              onClick={() => navigate('/markets')}
            >
              Browse Markets
            </button>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div style={{ marginTop: '60px' }}>
      <div className="card">
        <h2>Trade Stocks</h2>
        
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
          <div className="alert" style={{ backgroundColor: '#d4edda', color: '#155724', border: '1px solid #c3e6cb' }}>
            {message}
          </div>
        )}

        <form onSubmit={handleSubmit}>
          <div className="form-group">
            <label htmlFor="symbol">Stock Symbol</label>
            <input
              type="text"
              id="symbol"
              className="form-control"
              value={symbol}
              onChange={handleSymbolChange}
              placeholder="e.g., AAPL, GOOGL, MSFT"
              required
              disabled={loading}
              maxLength={10}
            />
          </div>

          <div className="form-group">
            <label htmlFor="quantity">Quantity</label>
            <input
              type="number"
              id="quantity"
              className="form-control"
              value={quantity}
              onChange={handleQuantityChange}
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

          <button
            type="submit"
            className={`btn ${tradeType === 'buy' ? 'btn-success' : 'btn-warning'}`}
            style={{ width: '100%' }}
            disabled={loading}
          >
            {loading ? `${tradeType === 'buy' ? 'Buying' : 'Selling'}...` : `${tradeType === 'buy' ? 'Buy' : 'Sell'} Stock`}
          </button>
        </form>

        {/* User Balance Display */}
        {user && (
          <div style={{ marginTop: '20px', padding: '10px', backgroundColor: '#f8f9fa', borderRadius: '4px' }}>
            <small style={{ color: '#666' }}>
              Current Balance: <strong>${user.balance?.toFixed(2) || '0.00'}</strong>
            </small>
          </div>
        )}
      </div>
    </div>
  );
};

export default Trade;

