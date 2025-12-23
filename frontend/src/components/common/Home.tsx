import React from 'react';
import { Link } from 'react-router-dom';

const Home: React.FC = () => {
  return (
    <div style={{ textAlign: 'center', marginTop: '60px' }}>
      <div className="card">
        <h1 style={{ fontSize: '36px', marginBottom: '20px' }}>
          Welcome to PaperTrader
        </h1>
        <p style={{ fontSize: '18px', color: '#666', marginBottom: '32px' }}>
          A simple and secure platform for paper trading. Practice your trading skills
          without risking real money.
        </p>
        
        <div style={{ display: 'flex', gap: '16px', justifyContent: 'center', flexWrap: 'wrap' }}>
          <Link to="/register" className="btn btn-primary">
            Get Started
          </Link>
          <Link to="/login" className="btn btn-secondary">
            Sign In
          </Link>
        </div>
        
        <div style={{ marginTop: '40px', padding: '20px', background: '#f8f9fa', borderRadius: '8px' }}>
          <h3 style={{ color: '#667eea', marginBottom: '16px' }}>Features</h3>
          <ul style={{ textAlign: 'left', listStyle: 'none', padding: 0 }}>
            <li style={{ marginBottom: '8px', padding: '8px 0' }}>✓ Secure user authentication</li>
            <li style={{ marginBottom: '8px', padding: '8px 0' }}>✓ Real-time market data</li>
            <li style={{ marginBottom: '8px', padding: '8px 0' }}>✓ Paper trading simulation</li>
            <li style={{ marginBottom: '8px', padding: '8px 0' }}>✓ Portfolio tracking</li>
          </ul>
        </div>
      </div>
    </div>
  );
};

export default Home;

