import React from 'react';
import { Link, NavLink, useLocation, useNavigate } from 'react-router-dom';
import { User } from '../../types';
import Wordmark from '../primitives/Wordmark';
import Button from '../primitives/Button';
import ThemeToggle from './ThemeToggle';

interface NavbarProps {
  isAuthenticated: boolean;
  user: User | null;
  onLogout: () => Promise<void>;
}

const initials = (user: User | null): string => {
  if (!user) return '·';
  const source = (user.email || '').trim();
  if (!source) return '·';
  const local = source.split('@')[0];
  const parts = local.split(/[._-]/).filter(Boolean);
  if (parts.length >= 2) {
    return (parts[0][0] + parts[1][0]).toUpperCase();
  }
  return local.slice(0, 2).toUpperCase();
};

const Navbar: React.FC<NavbarProps> = ({ isAuthenticated, user, onLogout }) => {
  const navigate = useNavigate();
  const location = useLocation();

  const handleLogoutClick = async () => {
    await onLogout();
    navigate('/');
  };

  // Marketing nav: shown on /, /login, /register, /verify-email
  const marketingPaths = ['/', '/login', '/register', '/verify-email'];
  const useMarketingNav = !isAuthenticated || marketingPaths.includes(location.pathname);

  if (useMarketingNav) {
    return (
      <nav className="top-nav" aria-label="Primary">
        <Wordmark to="/" />
        <div className="nav-links">
          <Link to="/markets">Markets</Link>
          <Link to="/calculator">Tools</Link>
          {isAuthenticated ? (
            <>
              <Link to="/dashboard">Dashboard</Link>
              <button
                type="button"
                onClick={handleLogoutClick}
                className="btn btn-ghost"
                style={{ height: 34, padding: '0 12px' }}
              >
                Log out
              </button>
            </>
          ) : (
            <>
              <Link to="/login" style={{ color: 'var(--ink)' }}>
                Log in
              </Link>
              <Link to="/register" className="btn btn-primary" style={{ height: 34, padding: '0 14px' }}>
                Sign up
              </Link>
            </>
          )}
          <ThemeToggle />
        </div>
      </nav>
    );
  }

  // App nav: signed in on app surfaces
  const tradeDisabled = !user?.email_verified;

  return (
    <nav className="app-nav" aria-label="App">
      <div>
        <Wordmark to="/dashboard" size={16} />
      </div>
      <div className="center">
        <NavLink
          to="/dashboard"
          className={({ isActive }) => `nav-item${isActive ? ' active' : ''}`}
        >
          Dashboard
        </NavLink>
        <NavLink
          to="/markets"
          className={({ isActive }) => `nav-item${isActive ? ' active' : ''}`}
        >
          Markets
        </NavLink>
        {tradeDisabled ? (
          <span
            className="nav-item disabled"
            title="Verify your email to trade"
            aria-disabled="true"
          >
            Trade
          </span>
        ) : (
          <NavLink
            to="/trade"
            className={({ isActive }) => `nav-item${isActive ? ' active' : ''}`}
          >
            Trade
          </NavLink>
        )}
        <NavLink
          to="/history"
          className={({ isActive }) => `nav-item${isActive ? ' active' : ''}`}
        >
          History
        </NavLink>
        <NavLink
          to="/calculator"
          className={({ isActive }) => `nav-item${isActive ? ' active' : ''}`}
        >
          Tools
        </NavLink>
      </div>
      <div className="right">
        <ThemeToggle />
        <span className="avatar" aria-label={user?.email ?? 'Account'}>
          {initials(user)}
        </span>
        <Button variant="ghost" size="sm" onClick={handleLogoutClick}>
          Log out
        </Button>
      </div>
    </nav>
  );
};

export default Navbar;
