/**
 * Error handling types
 */

/**
 * API Error response structure
 */
export interface ApiError {
  success: false;
  message: string;
  error?: string;
  code?: string;
}

/**
 * Application error types
 */
export type AppError =
  | { type: 'NETWORK_ERROR'; message: string }
  | { type: 'VALIDATION_ERROR'; message: string; field?: string }
  | { type: 'AUTH_ERROR'; message: string }
  | { type: 'API_ERROR'; message: string; status?: number }
  | { type: 'UNKNOWN_ERROR'; message: string };

/**
 * Check if error is an API error
 */
export const isApiError = (error: unknown): error is ApiError => {
  return (
    typeof error === 'object' &&
    error !== null &&
    'success' in error &&
    (error as ApiError).success === false &&
    'message' in error
  );
};

/**
 * Extract error message from unknown error type
 */
export const getErrorMessage = (error: unknown): string => {
  if (error instanceof Error) {
    return error.message;
  }
  if (isApiError(error)) {
    return error.message;
  }
  if (typeof error === 'string') {
    return error;
  }
  return 'An unknown error occurred';
};

