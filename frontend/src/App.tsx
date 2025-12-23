import React from 'react';
import { BrowserRouter as Router, Routes, Route, Navigate } from 'react-router-dom';
import Navbar from './components/layout/Navbar';
import Login from './components/auth/Login';
import Register from './components/auth/Register';
import Dashboard from './components/trading/Dashboard';
import Home from './components/common/Home';
import Trade from './components/trading/Trade';
import Calculator from './components/tools/Calculator';
import CompoundInterest from './components/tools/CompoundInterest';
import { useAuth } from './hooks/useAuth';
import './App.css';

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
    <Router>
      <div className="App">
        <Navbar 
          isAuthenticated={isAuthenticated} 
          user={user}
          onLogout={logout}
        />
        <div className="container">
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
              path="/dashboard" 
              element={
                isAuthenticated && user ? 
                <Dashboard user={user} /> : 
                <Navigate to="/login" replace />
              } 
            />
            <Route 
              path="/trade" 
              element={
                isAuthenticated && user ? 
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
};

export default App;

