import React, { useState, useEffect } from 'react';
import { BrowserRouter as Router, Routes, Route, Navigate, Link } from 'react-router-dom';
import Navbar from './components/Navbar';
import Login from './components/Login';
import Register from './components/Register';
import Dashboard from './components/Dashboard';
import Home from './components/Home';
import Trade from './components/Trade';
import './App.css';

function App() {
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [user, setUser] = useState(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    checkAuthStatus();
  }, []);

  const checkAuthStatus = async () => {
    try {
      const response = await fetch('/api/auth/check', {
        credentials: 'include'
      });
      const data = await response.json();
      
      if (data.success) {
        setIsAuthenticated(true);
        // Fetch user profile
        const profileResponse = await fetch('/api/auth/profile', {
          credentials: 'include'
        });
        if (profileResponse.ok) {
          const userData = await profileResponse.json();
          setUser(userData);
        }
      }
    } catch (error) {
      console.error('Auth check failed:', error);
    } finally {
      setLoading(false);
    }
  };

  const handleLogin = (userData) => {
    setIsAuthenticated(true);
    setUser(userData);
  };

  const handleLogout = () => {
    setIsAuthenticated(false);
    setUser(null);
  };

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
    <Router>
      <div className="App">
        <Navbar 
          isAuthenticated={isAuthenticated} 
          user={user}
          onLogout={handleLogout}
        />
        <div className="container">
          <Routes>
            <Route path="/" element={<Home />} />
            <Route 
              path="/login" 
              element={
                isAuthenticated ? 
                <Navigate to="/dashboard" replace /> : 
                <Login onLogin={handleLogin} />
              } 
            />
            <Route 
              path="/register" 
              element={
                isAuthenticated ? 
                <Navigate to="/dashboard" replace /> : 
                <Register onLogin={handleLogin} />
              } 
            />
            <Route 
              path="/dashboard" 
              element={
                isAuthenticated ? 
                <Dashboard user={user} /> : 
                <Navigate to="/login" replace />
              } 
            />
            <Route 
              path="/trade" 
              element={
                isAuthenticated ? 
                <Trade user={user} /> : 
                <Navigate to="/login" replace />
              } 
            />
          </Routes>
        </div>
      </div>
    </Router>
  );
}

export default App;

