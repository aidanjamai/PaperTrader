/**
 * Authentication hook
 * 
 * Manages user authentication state and provides auth-related methods
 */

import { useState, useEffect, useCallback } from 'react';
import { User, AuthResponse } from '../types';
import { apiRequest } from '../services/api';

interface UseAuthReturn {
  user: User | null;
  isAuthenticated: boolean;
  loading: boolean;
  login: (user: User) => void;
  logout: () => Promise<void>;
  checkAuth: () => Promise<void>;
}

/**
 * Custom hook for authentication state management
 */
export const useAuth = (): UseAuthReturn => {
  const [user, setUser] = useState<User | null>(null);
  const [isAuthenticated, setIsAuthenticated] = useState<boolean>(false);
  const [loading, setLoading] = useState<boolean>(true);

  /**
   * Check authentication status with the backend
   */
  const checkAuth = useCallback(async () => {
    try {
      const response = await apiRequest<AuthResponse>('/account/auth');
      
      if (response.ok) {
        const data = await response.json() as AuthResponse;
        if (data.success) {
          setIsAuthenticated(true);
          // Fetch user profile
          const profileResponse = await apiRequest<User>('/account/profile');
          if (profileResponse.ok) {
            const userData = await profileResponse.json() as User;
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
  }, []);

  /**
   * Initialize auth state on mount
   */
  useEffect(() => {
    checkAuth();
  }, [checkAuth]);

  /**
   * Login handler - sets user and authenticated state
   */
  const login = useCallback((userData: User) => {
    setIsAuthenticated(true);
    setUser(userData);
  }, []);

  /**
   * Logout handler - clears auth state and calls backend
   */
  const logout = useCallback(async () => {
    try {
      await apiRequest('/account/logout', { method: 'POST' });
    } catch (error) {
      console.error('Logout failed', error);
    } finally {
      localStorage.removeItem('token');
      localStorage.removeItem('user');
      setIsAuthenticated(false);
      setUser(null);
    }
  }, []);

  return {
    user,
    isAuthenticated,
    loading,
    login,
    logout,
    checkAuth,
  };
};

