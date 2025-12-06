import React, { useState } from 'react';

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
            // Get user ID from localStorage
            const storedUser = localStorage.getItem('user');
            let userId;
            
            if (user && user.id) {
                userId = user.id;
            } else if (storedUser) {
                const parsedUser = JSON.parse(storedUser);
                userId = parsedUser.id;
            } else {
                setError('User not found. Please log in again.');
                setLoading(false);
                return;
            }

            const endpoint = tradeType === 'buy' ? '/api/investments/buy' : '/api/investments/sell';
            
            const response = await fetch(endpoint, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({
                    userId: userId,
                    symbol: symbol.toUpperCase().trim(),
                    quantity: quantityNum
                })
            });

            if (response.ok) {
                const result = await response.json();
                setMessage(`${tradeType === 'buy' ? 'Bought' : 'Sold'} ${quantityNum} shares of ${symbol.toUpperCase()} successfully!`);
                setSymbol('');
                setQuantity('');
            } else {
                const errorData = await response.text();
                setError(errorData || `${tradeType === 'buy' ? 'Buy' : 'Sell'} failed`);
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