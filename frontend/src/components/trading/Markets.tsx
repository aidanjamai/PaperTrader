import React from 'react';

/**
 * Markets component
 * 
 * TODO: Implement market listing with:
 * - 10 most popular stocks
 * - 5 most popular index funds
 * - 5 most popular crypto
 * - Quick stats (daily change, percentage)
 * - Click to view stock details
 */
const Markets: React.FC = () => {
  return (
    <div style={{ marginTop: '60px' }}>
      <div className="card">
        <h2>Markets</h2>
        <p style={{ color: '#666' }}>
          Market listings coming soon. This will show:
        </p>
        <ul>
          <li>10 most popular stocks</li>
          <li>5 most popular index funds</li>
          <li>5 most popular crypto</li>
          <li>Quick stats (daily change, percentage)</li>
          <li>Click to view detailed stock information</li>
        </ul>
      </div>
    </div>
  );
};

export default Markets;

