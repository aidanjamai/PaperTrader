import React from 'react';
import { BrowserRouter as Router, Routes, Route, Navigate } from 'react-router-dom';
import { GoogleOAuthProvider } from '@react-oauth/google';
import Navbar from './components/layout/Navbar';
import Login from './components/auth/Login';
import Register from './components/auth/Register';
import VerifyEmail from './components/auth/VerifyEmail';
import EmailVerificationBanner from './components/auth/EmailVerificationBanner';
import Dashboard from './components/trading/Dashboard';
import Home from './components/common/Home';
import Trade from './components/trading/Trade';
import History from './components/trading/History';
import Markets from './components/trading/Markets';
import Calculator from './components/tools/Calculator';
import CompoundInterest from './components/tools/CompoundInterest';
import { useAuth } from './hooks/useAuth';
import { ThemeProvider } from './hooks/useTheme';
import './App.css';

const GOOGLE_CLIENT_ID = process.env.REACT_APP_GOOGLE_CLIENT_ID || '';

const App: React.FC = () => {
  const { user, isAuthenticated, loading, login, logout } = useAuth();

  if (loading) {
    return (
      <ThemeProvider>
        <div className="app-loading">Loading…</div>
      </ThemeProvider>
    );
  }

  return (
    <ThemeProvider>
      <GoogleOAuthProvider clientId={GOOGLE_CLIENT_ID}>
        <Router>
          <div className="app-shell">
            <Navbar
              isAuthenticated={isAuthenticated}
              user={user}
              onLogout={logout}
            />
            <main className="app-main">
              {isAuthenticated && user && !user.email_verified && (
                <div className="container">
                  <EmailVerificationBanner email={user.email} />
                </div>
              )}
              <Routes>
                  <Route path="/" element={<Home />} />
                  <Route
                    path="/login"
                    element={
                      isAuthenticated ? (
                        <Navigate to="/dashboard" replace />
                      ) : (
                        <Login onLogin={login} />
                      )
                    }
                  />
                  <Route
                    path="/register"
                    element={
                      isAuthenticated ? (
                        <Navigate to="/dashboard" replace />
                      ) : (
                        <Register onLogin={login} />
                      )
                    }
                  />
                  <Route path="/verify-email" element={<VerifyEmail />} />
                  <Route
                    path="/dashboard"
                    element={
                      isAuthenticated && user ? (
                        <Dashboard />
                      ) : (
                        <Navigate to="/login" replace />
                      )
                    }
                  />
                  <Route
                    path="/trade"
                    element={
                      isAuthenticated && user ? (
                        <Trade />
                      ) : (
                        <Navigate to="/login" replace />
                      )
                    }
                  />
                  <Route
                    path="/markets"
                    element={
                      isAuthenticated && user ? (
                        <Markets />
                      ) : (
                        <Navigate to="/login" replace />
                      )
                    }
                  />
                  <Route
                    path="/history"
                    element={
                      isAuthenticated && user ? (
                        <History />
                      ) : (
                        <Navigate to="/login" replace />
                      )
                    }
                  />
                  <Route path="/calculator" element={<Calculator />} />
                  <Route path="/compound-interest" element={<CompoundInterest />} />
                </Routes>
            </main>
          </div>
        </Router>
      </GoogleOAuthProvider>
    </ThemeProvider>
  );
};

export default App;
