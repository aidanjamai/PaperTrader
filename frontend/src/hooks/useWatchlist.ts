import { useCallback, useState } from 'react';
import { apiRequest } from '../services/api';
import { WatchlistEntry, WatchlistResponse } from '../types';

interface UseWatchlistReturn {
  items: WatchlistEntry[];
  loading: boolean;
  error: string | null;
  fetchWatchlist: () => Promise<void>;
  addSymbol: (symbol: string) => Promise<void>;
  removeSymbol: (symbol: string) => Promise<void>;
}

const parseError = async (response: Response, fallback: string): Promise<string> => {
  try {
    const body = await response.json();
    if (body && typeof body.message === 'string') return body.message;
  } catch {
    // fall through
  }
  return fallback;
};

export const useWatchlist = (): UseWatchlistReturn => {
  const [items, setItems] = useState<WatchlistEntry[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetchWatchlist = useCallback(async (): Promise<void> => {
    setError(null);
    setLoading(true);
    try {
      const response = await apiRequest<WatchlistResponse>('/watchlist');
      if (!response.ok) {
        setError(await parseError(response, `Failed to load watchlist (${response.status})`));
        return;
      }
      const data = (await response.json()) as WatchlistResponse;
      setItems(Array.isArray(data?.items) ? data.items : []);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load watchlist');
    } finally {
      setLoading(false);
    }
  }, []);

  const addSymbol = useCallback(async (symbol: string): Promise<void> => {
    const response = await apiRequest('/watchlist', {
      method: 'POST',
      body: JSON.stringify({ symbol }),
    });
    if (!response.ok) {
      throw new Error(await parseError(response, 'Failed to add symbol'));
    }
    const entry = (await response.json()) as WatchlistEntry;
    setItems((prev) => {
      if (prev.some((e) => e.symbol === entry.symbol)) return prev;
      return [...prev, entry].sort((a, b) => a.symbol.localeCompare(b.symbol));
    });
  }, []);

  const removeSymbol = useCallback(async (symbol: string): Promise<void> => {
    const response = await apiRequest(`/watchlist/${encodeURIComponent(symbol)}`, {
      method: 'DELETE',
    });
    if (!response.ok && response.status !== 404) {
      throw new Error(await parseError(response, 'Failed to remove symbol'));
    }
    setItems((prev) => prev.filter((e) => e.symbol !== symbol));
  }, []);

  return { items, loading, error, fetchWatchlist, addSymbol, removeSymbol };
};
