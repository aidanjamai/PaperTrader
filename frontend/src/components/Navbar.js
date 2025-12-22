import React from 'react';
import { Link, useNavigate } from 'react-router-dom';

function Navbar({ isAuthenticated, user, onLogout }) {
  const navigate = useNavigate();

  const handleLogoutClick = async () => {
    await onLogout();
    navigate('/');
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
                  {/* Welcome, {user?.email}! */}
                </span>
                <Link to="/dashboard">Dashboard</Link>
                <Link to="/trade">Trade</Link>
                <Link to="/calculator">Calculator</Link>
                <Link to="/compound-interest">Compound Interest</Link>
                <button 
                  onClick={handleLogoutClick}
                  className="btn btn-secondary"
                  style={{ background: 'transparent', border: 'none', color: '#667eea', cursor: 'pointer' }}
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
