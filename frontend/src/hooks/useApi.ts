/**
 * Generic API hook
 * 
 * Provides a reusable hook for API calls with loading and error states
 */

import { useState, useCallback, useEffect, useRef } from 'react';
import { apiRequest, apiRequestJson } from '../services/api';

interface UseApiReturn<T> {
  data: T | null;
  loading: boolean;
  error: string | null;
  execute: () => Promise<void>;
  reset: () => void;
}

interface UseApiOptions<T> {
  endpoint: string;
  options?: RequestInit;
  autoExecute?: boolean;
  parser?: (response: Response) => Promise<T>;
}

/**
 * Generic hook for API calls with loading and error states
 * 
 * @template T - The expected response type
 * @param config - Configuration object
 * @returns Object with data, loading, error, execute, and reset
 */
export const useApi = <T = unknown>(
  config: UseApiOptions<T>
): UseApiReturn<T> => {
  const { endpoint, options, autoExecute = false, parser } = config;
  
  const [data, setData] = useState<T | null>(null);
  const [loading, setLoading] = useState<boolean>(autoExecute);
  const [error, setError] = useState<string | null>(null);

  const execute = useCallback(async () => {
    setLoading(true);
    setError(null);

    try {
      let result: T;
      
      if (parser) {
        const response = await apiRequest(endpoint, options);
        result = await parser(response);
      } else {
        result = await apiRequestJson<T>(endpoint, options);
      }
      
      setData(result);
    } catch (err) {
      const errorMessage = err instanceof Error ? err.message : 'An error occurred';
      setError(errorMessage);
      setData(null);
    } finally {
      setLoading(false);
    }
  }, [endpoint, options, parser]);

  const reset = useCallback(() => {
    setData(null);
    setError(null);
    setLoading(false);
  }, []);

  // Store execute function in ref to avoid re-triggering useEffect
  const executeRef = useRef(execute);
  executeRef.current = execute;

  // Auto-execute if configured
  useEffect(() => {
    if (autoExecute) {
      executeRef.current();
    }
  }, [autoExecute]);

  return {
    data,
    loading,
    error,
    execute,
    reset,
  };
};


