import React, { useState } from 'react';
import { apiRequest } from '../utils/api';

function Trade({ user }) {
    const [tradeType, setTradeType] = useState('buy'); // 'buy' or 'sell'
    const [symbol, setSymbol] = useState('');
    const [quantity, setQuantity] = useState('');
    const [loading, setLoading] = useState(false);
    const [message, setMessage] = useState('');
    const [error, setError] = useState('');

    const handleSubmit = async (e) => {
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

        const quantityNum = parseInt(quantity);
        if (isNaN(quantityNum) || quantityNum <= 0) {
            setError('Quantity must be a positive whole number');
            setLoading(false);
            return;
        }

        try {
            // Use apiRequest to ensure auth headers are included
            const endpoint = tradeType === 'buy' ? '/investments/buy' : '/investments/sell';
            
            const response = await apiRequest(endpoint, {
                method: 'POST',
                body: JSON.stringify({
                    symbol: symbol.toUpperCase().trim(),
                    quantity: quantityNum
                    // userId is handled by backend from token
                })
            });

            if (response.ok) {
                await response.json(); // Response processed, result not needed
                setMessage(`${tradeType === 'buy' ? 'Bought' : 'Sold'} ${quantityNum} shares of ${symbol.toUpperCase()} successfully!`);
                setSymbol('');
                setQuantity('');
                // Optional: Trigger a refresh of user balance/portfolio here if we had a global context
            } else {
                const errorData = await response.text();
                // Try to parse JSON error if possible
                try {
                    const jsonError = JSON.parse(errorData);
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
                            onChange={(e) => setSymbol(e.target.value.toUpperCase())}
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
}

export default Trade;
