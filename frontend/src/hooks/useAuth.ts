/**
 * Authentication hook
 * 
 * Manages user authentication state and provides auth-related methods
 * Uses a singleton pattern to prevent multiple auth checks across components
 */

import { useState, useEffect, useCallback, useRef } from 'react';
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

// Shared state across all useAuth instances (singleton pattern)
let globalAuthState: {
  user: User | null;
  isAuthenticated: boolean;
  loading: boolean;
  checkPromise: Promise<void> | null;
} = {
  user: null,
  isAuthenticated: false,
  loading: true,
  checkPromise: null,
};

// Cache keys
const AUTH_CACHE_KEY = 'papertrader_auth_user';
const AUTH_TIMESTAMP_KEY = 'papertrader_auth_timestamp';
const AUTH_CACHE_DURATION = 5 * 60 * 1000; // 5 minutes

/**
 * Custom hook for authentication state management
 */
export const useAuth = (): UseAuthReturn => {
  const [user, setUser] = useState<User | null>(globalAuthState.user);
  const [isAuthenticated, setIsAuthenticated] = useState<boolean>(globalAuthState.isAuthenticated);
  const [loading, setLoading] = useState<boolean>(globalAuthState.loading);
  const isMountedRef = useRef(true);

  /**
   * Verify auth in background without blocking UI
   */
  const verifyAuthInBackground = useCallback(async () => {
    try {
      const response = await apiRequest<AuthResponse>('/account/auth');
      if (response.ok) {
        const data = await response.json() as AuthResponse;
        if (!data.success) {
          // Auth expired, clear cache
          globalAuthState.isAuthenticated = false;
          globalAuthState.user = null;
          localStorage.removeItem(AUTH_CACHE_KEY);
          localStorage.removeItem(AUTH_TIMESTAMP_KEY);
          
          if (isMountedRef.current) {
            setIsAuthenticated(false);
            setUser(null);
          }
        }
      }
    } catch (error) {
      // Silently fail in background check
      console.warn('[useAuth] Background auth check failed:', error);
    }
  }, []);

  /**
   * Check authentication status with the backend
   * Uses singleton pattern to prevent multiple simultaneous requests
   */
  const checkAuth = useCallback(async () => {
    // If there's already a check in progress, wait for it
    if (globalAuthState.checkPromise) {
      await globalAuthState.checkPromise;
      if (isMountedRef.current) {
        setUser(globalAuthState.user);
        setIsAuthenticated(globalAuthState.isAuthenticated);
        setLoading(globalAuthState.loading);
      }
      return;
    }

    // Check cache first
    try {
      const cachedUser = localStorage.getItem(AUTH_CACHE_KEY);
      const cachedTimestamp = localStorage.getItem(AUTH_TIMESTAMP_KEY);
      
      if (cachedUser && cachedTimestamp) {
        const timestamp = parseInt(cachedTimestamp, 10);
        const now = Date.now();
        
        // Use cached data if less than 5 minutes old
        if (now - timestamp < AUTH_CACHE_DURATION) {
          const userData = JSON.parse(cachedUser) as User;
          globalAuthState.user = userData;
          globalAuthState.isAuthenticated = true;
          globalAuthState.loading = false;
          
          if (isMountedRef.current) {
            setUser(userData);
            setIsAuthenticated(true);
            setLoading(false);
          }
          
          // Still verify auth in background (non-blocking)
          verifyAuthInBackground();
          return;
        }
      }
    } catch (error) {
      console.warn('[useAuth] Error reading cache:', error);
    }

    // Create a promise for the auth check
    globalAuthState.checkPromise = (async () => {
      try {
        const response = await apiRequest<AuthResponse>('/account/auth');
        
        if (response.ok) {
          const data = await response.json() as AuthResponse;
          if (data.success) {
            // Fetch user profile only if not cached or cache expired
            const profileResponse = await apiRequest<User>('/account/profile');
            if (profileResponse.ok) {
              const userData = await profileResponse.json() as User;
              
              // Update global state
              globalAuthState.user = userData;
              globalAuthState.isAuthenticated = true;
              
              // Cache user data
              try {
                localStorage.setItem(AUTH_CACHE_KEY, JSON.stringify(userData));
                localStorage.setItem(AUTH_TIMESTAMP_KEY, Date.now().toString());
              } catch (error) {
                console.warn('[useAuth] Error caching user data:', error);
              }
              
              if (isMountedRef.current) {
                setUser(userData);
                setIsAuthenticated(true);
              }
            }
          } else {
            globalAuthState.isAuthenticated = false;
            globalAuthState.user = null;
            localStorage.removeItem(AUTH_CACHE_KEY);
            localStorage.removeItem(AUTH_TIMESTAMP_KEY);
            
            if (isMountedRef.current) {
              setIsAuthenticated(false);
              setUser(null);
            }
          }
        } else {
          globalAuthState.isAuthenticated = false;
          globalAuthState.user = null;
          localStorage.removeItem(AUTH_CACHE_KEY);
          localStorage.removeItem(AUTH_TIMESTAMP_KEY);
          
          if (isMountedRef.current) {
            setIsAuthenticated(false);
            setUser(null);
          }
        }
      } catch (error) {
        console.error('[useAuth] Auth check failed:', error);
        globalAuthState.isAuthenticated = false;
        globalAuthState.user = null;
        
        if (isMountedRef.current) {
          setIsAuthenticated(false);
          setUser(null);
        }
      } finally {
        globalAuthState.loading = false;
        globalAuthState.checkPromise = null;
        
        if (isMountedRef.current) {
          setLoading(false);
        }
      }
    })();

    await globalAuthState.checkPromise;
  }, [verifyAuthInBackground]);

  /**
   * Initialize auth state on mount
   * Only runs once globally, not per component
   */
  useEffect(() => {
    // Only check auth if not already checked
    if (globalAuthState.checkPromise === null && !globalAuthState.user) {
      checkAuth();
    } else {
      // Sync with global state
      setUser(globalAuthState.user);
      setIsAuthenticated(globalAuthState.isAuthenticated);
      setLoading(globalAuthState.loading);
    }

    return () => {
      isMountedRef.current = false;
    };
  }, [checkAuth]);

  /**
   * Login handler - sets user and authenticated state
   */
  const login = useCallback((userData: User) => {
    globalAuthState.user = userData;
    globalAuthState.isAuthenticated = true;
    globalAuthState.loading = false;
    
    // Cache user data
    try {
      localStorage.setItem(AUTH_CACHE_KEY, JSON.stringify(userData));
      localStorage.setItem(AUTH_TIMESTAMP_KEY, Date.now().toString());
    } catch (error) {
      console.warn('[useAuth] Error caching user data:', error);
    }
    
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
      console.error('[useAuth] Logout failed', error);
    } finally {
      localStorage.removeItem('token');
      localStorage.removeItem('user');
      localStorage.removeItem(AUTH_CACHE_KEY);
      localStorage.removeItem(AUTH_TIMESTAMP_KEY);
      
      globalAuthState.user = null;
      globalAuthState.isAuthenticated = false;
      globalAuthState.loading = false;
      
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

