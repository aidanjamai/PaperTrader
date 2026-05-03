/**
 * API Type Definitions
 * 
 * These types match the backend DTOs for type-safe API communication
 */

/**
 * User account information
 */
export interface User {
  id: string;
  email: string;
  created_at: string;
  balance: number;
  email_verified?: boolean;
}

/**
 * User's stock holding in portfolio
 */
export interface UserStock {
  id: string;
  user_id: string;
  symbol: string;
  quantity: number;
  avg_price: number;
  total: number;
  current_stock_price: number;
  created_at: string;
  updated_at: string;
}

/**
 * Authentication response from login/register endpoints
 */
export interface AuthResponse {
  success: boolean;
  message: string;
  user?: User;
  token?: string;
}

/**
 * Generic error response from API
 */
export interface ErrorResponse {
  success: boolean;
  message: string;
  error?: string;
}

/**
 * Login request payload
 */
export interface LoginRequest {
  email: string;
  password: string;
}

/**
 * Registration request payload
 */
export interface RegisterRequest {
  email: string;
  password: string;
}

/**
 * Buy stock request payload
 */
export interface BuyStockRequest {
  symbol: string;
  quantity: number;
}

/**
 * Sell stock request payload
 */
export interface SellStockRequest {
  symbol: string;
  quantity: number;
}

/**
 * Trade response from buy/sell operations
 */
export interface TradeResponse {
  id: string;
  symbol: string;
  action: string;
  quantity: number;
  price: number;
  total: number;
  executed_at: string;
}

/**
 * A single trade row as returned by GET /investments/history.
 * total is computed server-side as quantity * price.
 * executed_at is an ISO 8601 timestamp.
 */
export interface Trade {
  id: string;
  user_id: string;
  symbol: string;
  action: 'BUY' | 'SELL';
  quantity: number;
  price: number;
  total: number;
  executed_at: string;
  status: string;
}

/**
 * Paginated response from GET /investments/history.
 * total is the count of all trades matching the filter — independent of limit/offset.
 */
export interface TradeHistoryResponse {
  trades: Trade[];
  total: number;
  limit: number;
  offset: number;
}

/**
 * Stock price response
 */
export interface StockResponse {
  symbol: string;
  date: string;
  price: number;
}

/**
 * Historical stock data response
 */
export interface HistoricalDataResponse {
  symbol: string;
  date: string;
  previous_price: number;
  price: number;
  volume: number;
  change: number;
  change_percentage: number;
}

/**
 * A single point in a stock-history time series.
 * date is ISO YYYY-MM-DD; close is in dollars (2dp on the wire).
 */
export interface HistoricalSeriesPoint {
  date: string;
  close: number;
}

/**
 * Response shape from GET /market/stock/historical/series.
 */
export interface HistoricalSeriesResponse {
  symbol: string;
  from: string;
  to: string;
  points: HistoricalSeriesPoint[];
}

/**
 * A single watchlist entry as returned by GET /watchlist.
 * has_price is false when the price lookup failed (treat price/change as unknown).
 */
export interface WatchlistEntry {
  id: string;
  symbol: string;
  created_at: string;
  price: number;
  change: number;
  change_percentage: number;
  has_price: boolean;
}

export interface WatchlistResponse {
  items: WatchlistEntry[];
}

