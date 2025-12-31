import { useState, useCallback } from 'react';
import { apiRequest } from '../services/api';
import { UserStock } from '../types';

interface UsePortfolioReturn {
  stocks: UserStock[];
  loading: boolean;
  error: string | null;
  fetchPortfolio: () => Promise<void>;
}

/**
 * Custom hook for fetching and managing user portfolio
 * 
 * Provides portfolio data, loading state, error handling, and refetch capability
 */
export const usePortfolio = (): UsePortfolioReturn => {
  const [stocks, setStocks] = useState<UserStock[]>([]);
  const [loading, setLoading] = useState<boolean>(false);
  const [error, setError] = useState<string | null>(null);

  const fetchPortfolio = useCallback(async (): Promise<void> => {
    setError(null);
    setLoading(true);
    
    try {
      const response = await apiRequest<UserStock[]>('/investments');
      
      // Check response status first
      if (!response.ok) {
        const errorText = await response.text().catch(() => 'Unknown error');
        
        if (response.status === 401) {
          setError('Authentication required. Please log in again.');
        } else if (response.status === 404) {
          setError('API endpoint not found. Please check that the server is running and the route is correct.');
        } else if (response.status >= 500) {
          setError(`Server error (${response.status}). Please try again later.`);
        } else {
          setError(`Failed to load portfolio (${response.status}): ${errorText}`);
        }
        setLoading(false);
        return;
      }
      
      // Check content-type
      const contentType = response.headers.get('content-type');
      if (contentType && !contentType.includes('application/json')) {
        setError('Invalid response format from server. Expected JSON.');
        setLoading(false);
        return;
      }

      // Parse and validate response
      const data = await response.json();
      
      // Validate response is an array
      if (!Array.isArray(data)) {
        setError('Invalid response format. Expected an array of stocks.');
        setLoading(false);
        return;
      }
      
      // Validate each stock object has required fields
      const validStocks: UserStock[] = [];
      for (const stock of data) {
        if (!stock || typeof stock !== 'object') {
          continue;
        }
        
        // Validate required fields
        if (!stock.symbol || typeof stock.quantity !== 'number' || typeof stock.avg_price !== 'number') {
          continue;
        }
        
        validStocks.push({
          id: stock.id || '',
          user_id: stock.user_id || '',
          symbol: stock.symbol,
          quantity: stock.quantity,
          avg_price: stock.avg_price,
          total: stock.total || stock.avg_price * stock.quantity,
          current_stock_price: stock.current_stock_price || 0,
          created_at: stock.created_at || '',
          updated_at: stock.updated_at || ''
        });
      }
      
      setStocks(validStocks);
      
    } catch (error) {
      if (error instanceof SyntaxError) {
        setError('Failed to parse server response. Please check the server is running correctly.');
      } else if (error instanceof Error) {
        setError(`Error loading portfolio: ${error.message}`);
      } else {
        setError('An unexpected error occurred while loading your portfolio.');
      }
    } finally {
      setLoading(false);
    }
  }, []);

  return {
    stocks,
    loading,
    error,
    fetchPortfolio,
  };
};