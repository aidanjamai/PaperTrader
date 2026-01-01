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
import Markets from './components/trading/Markets';
import Calculator from './components/tools/Calculator';
import CompoundInterest from './components/tools/CompoundInterest';
import { useAuth } from './hooks/useAuth';
import './App.css';

const GOOGLE_CLIENT_ID = process.env.REACT_APP_GOOGLE_CLIENT_ID || '';

const App: React.FC = () => {
  const { user, isAuthenticated, loading, login, logout } = useAuth();

  if (loading) {
    return (
      <div className="container" style={{ textAlign: 'center', marginTop: '100px' }}>
        <div className="card">
          <h2>Loading...</h2>
        </div>
      </div>
    );
  }

  return (
    <GoogleOAuthProvider clientId={GOOGLE_CLIENT_ID}>
      <Router>
        <div className="App">
        <Navbar 
          isAuthenticated={isAuthenticated} 
          user={user}
          onLogout={logout}
        />
        <div className="container">
          {isAuthenticated && user && !user.email_verified && (
            <EmailVerificationBanner email={user.email} />
          )}
          <Routes>
            <Route path="/" element={<Home />} />
            <Route 
              path="/login" 
              element={
                isAuthenticated ? 
                <Navigate to="/dashboard" replace /> : 
                <Login onLogin={login} />
              } 
            />
            <Route 
              path="/register" 
              element={
                isAuthenticated ? 
                <Navigate to="/dashboard" replace /> : 
                <Register onLogin={login} />
              } 
            />
            <Route 
              path="/verify-email" 
              element={<VerifyEmail />} 
            />
            <Route 
              path="/dashboard" 
              element={
                isAuthenticated && user ? 
                <Dashboard /> : 
                <Navigate to="/login" replace />
              } 
            />
            <Route 
              path="/trade" 
              element={
                isAuthenticated && user ? 
                <Trade /> : 
                <Navigate to="/login" replace />
              }
            />
            <Route 
              path="/markets" 
              element={
                isAuthenticated && user ? 
                <Markets /> : 
                <Navigate to="/login" replace />
              }
            />
            <Route 
              path="/calculator" 
              element={<Calculator />} 
            />
            <Route 
              path="/compound-interest" 
              element={<CompoundInterest />} 
            />
          </Routes>
        </div>
      </div>
      </Router>
    </GoogleOAuthProvider>
  );
};

export default App;

