import React from 'react';

function Dashboard({ user }) {
  return (
    <div className="dashboard">
      <h1>Welcome to Your Dashboard</h1>
      
      <div className="user-info">
        <h3>Account Information</h3>
        <p><strong>Email:</strong> {user?.email}</p>
        <p><strong>Member Since:</strong> {new Date(user?.created_at).toLocaleDateString()}</p>
        <p><strong>Account Balance:</strong> ${user?.balance?.toFixed(2) || '0.00'}</p>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(250px, 1fr))', gap: '20px' }}>
        <div style={{ background: '#f8f9fa', padding: '20px', borderRadius: '8px' }}>
          <h3 style={{ color: '#667eea', marginBottom: '12px' }}>Portfolio Value</h3>
          <p style={{ fontSize: '24px', fontWeight: 'bold', color: '#333' }}>$0.00</p>
          <p style={{ color: '#666', fontSize: '14px' }}>No positions yet</p>
        </div>

        <div style={{ background: '#f8f9fa', padding: '20px', borderRadius: '8px' }}>
          <h3 style={{ color: '#667eea', marginBottom: '12px' }}>Available Cash</h3>
          <p style={{ fontSize: '24px', fontWeight: 'bold', color: '#333' }}>${user?.balance?.toFixed(2) || '0.00'}</p>
          <p style={{ color: '#666', fontSize: '14px' }}>Current balance</p>
        </div>

        <div style={{ background: '#f8f9fa', padding: '20px', borderRadius: '8px' }}>
          <h3 style={{ color: '#667eea', marginBottom: '12px' }}>Total Positions</h3>
          <p style={{ fontSize: '24px', fontWeight: 'bold', color: '#333' }}>0</p>
          <p style={{ color: '#666', fontSize: '14px' }}>No open trades</p>
        </div>
      </div>

      <div style={{ marginTop: '32px', textAlign: 'center' }}>
        <h3 style={{ color: '#333', marginBottom: '16px' }}>Getting Started</h3>
        <p style={{ color: '#666', marginBottom: '24px' }}>
          Your account is ready! Start exploring the markets and practice your trading skills.
        </p>
        <div style={{ display: 'flex', gap: '16px', justifyContent: 'center', flexWrap: 'wrap' }}>
          <button className="btn btn-primary">
            Browse Markets
          </button>
          <button className="btn btn-secondary">
            View Tutorial
          </button>
        </div>
      </div>
    </div>
  );
}

export default Dashboard;
