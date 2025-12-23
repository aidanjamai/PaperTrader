/**
 * Type definitions index
 * 
 * Re-export all types for clean imports
 */

export * from './api';
export * from './errors';

/**
 * Trade action type
 */
export type TradeAction = 'buy' | 'sell';

/**
 * Common form data types
 */
export interface FormData {
  [key: string]: string | number | boolean;
}

