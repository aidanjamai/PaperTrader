import React from 'react';
import { Link, useNavigate } from 'react-router-dom';

function Navbar({ isAuthenticated, user, onLogout }) {
  const navigate = useNavigate();

  const handleLogout = async () => {
    try {
      await fetch('/api/auth/logout', {
        method: 'POST',
        credentials: 'include'
      });
      onLogout();
      navigate('/');
    } catch (error) {
      console.error('Logout failed:', error);
    }
  };

  return (
    <nav className="navbar">
      <div className="container">
        <div className="navbar-content">
          <Link to="/" className="navbar-brand">
            PaperTrader
          </Link>
          
          <div className="navbar-nav">
            {isAuthenticated ? (
              <>
                <span className="welcome-message">
                  Welcome, {user?.email}!
                </span>
                <Link to="/dashboard">Dashboard</Link>
                <Link to="/trade">Trade</Link>
                <button 
                  onClick={handleLogout}
                  className="btn btn-secondary"
                  style={{ background: 'transparent', border: 'none', color: '#667eea' }}
                >
                  Logout
                </button>
              </>
            ) : (
              <div className="auth-links">
                <Link to="/login" className="btn btn-secondary">
                  Login
                </Link>
                <Link to="/register" className="btn btn-primary">
                  Register
                </Link>
              </div>
            )}
          </div>
        </div>
      </div>
    </nav>
  );
}

export default Navbar;
