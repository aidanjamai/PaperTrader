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
  date: string;
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
 * Update balance request payload
 */
export interface UpdateBalanceRequest {
  balance: number;
}

/**
 * Get all users response
 */
export interface GetAllUsersResponse {
  users: User[];
}

