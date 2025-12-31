import React from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { User } from '../../types';

interface NavbarProps {
  isAuthenticated: boolean;
  user: User | null;
  onLogout: () => Promise<void>;
}

const Navbar: React.FC<NavbarProps> = ({ isAuthenticated, user, onLogout }) => {
  const navigate = useNavigate();

  const handleLogoutClick = async () => {
    await onLogout();
    navigate('/');
  };

  return (
    <nav className="navbar">
      <div className="navbar-container">
        <div className="navbar-content">
          <Link to="/" className="navbar-brand">
            PaperTrader
          </Link>
          
          <div className="navbar-nav">
            {isAuthenticated ? (
              <>
                
                <Link to="/dashboard">Dashboard</Link>
                {user?.email_verified ? (
                  <Link to="/trade">Trade</Link>
                ) : (
                  <span 
                    style={{ 
                      color: '#999', 
                      cursor: 'not-allowed', 
                      textDecoration: 'none',
                      opacity: 0.6
                    }}
                    title="Please verify your email to trade"
                  >
                    Trade
                  </span>
                )}
                <Link to="/markets">Markets</Link>
                <Link to="/calculator">Calculator</Link>
                <Link to="/compound-interest">Compound Interest</Link>
                <button 
                  onClick={handleLogoutClick}
                  className="btn btn-secondary"
                  style={{ background: 'transparent', border: 'none', color: '#667eea', cursor: 'pointer' }}
                  type="button"
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
};

export default Navbar;

