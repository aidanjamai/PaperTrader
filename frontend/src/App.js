import React, { useState, useEffect } from 'react';
import { BrowserRouter as Router, Routes, Route, Navigate } from 'react-router-dom';
import Navbar from './components/Navbar';
import Login from './components/Login';
import Register from './components/Register';
import Dashboard from './components/Dashboard';
import Home from './components/Home';
import Trade from './components/Trade';
import Calculator from './components/Calculator';
import CompoundInterest from './components/CompoundInterest';
import { apiRequest } from './utils/api';
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
      // The backend route is /api/account/auth
      const response = await apiRequest('/account/auth');
      
      if (response.ok) {
        const data = await response.json();
        if (data.success) {
          setIsAuthenticated(true);
          // Fetch user profile
          const profileResponse = await apiRequest('/account/profile');
          if (profileResponse.ok) {
            const userData = await profileResponse.json();
            setUser(userData);
          }
        } else {
          setIsAuthenticated(false);
          setUser(null);
        }
      } else {
        setIsAuthenticated(false);
        setUser(null);
      }
    } catch (error) {
      console.error('Auth check failed:', error);
      setIsAuthenticated(false);
      setUser(null);
    } finally {
      setLoading(false);
    }
  };

  const handleLogin = (userData) => {
    setIsAuthenticated(true);
    setUser(userData);
  };

  const handleLogout = async () => {
    try {
      // Call backend to clear cookie
      await apiRequest('/account/logout', { method: 'POST' });
    } catch (error) {
      console.error('Logout failed', error);
    } finally {
      // Clear local state regardless of backend success
      localStorage.removeItem('token');
      localStorage.removeItem('user');
      setIsAuthenticated(false);
      setUser(null);
    }
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
  );
}

export default App;
