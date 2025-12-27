/**
 * Type-safe API client
 * 
 * Provides type-safe HTTP requests to the backend API
 */

/**
 * API request options extending standard fetch options
 */
export interface ApiRequestOptions extends RequestInit {
  headers?: HeadersInit;
}

/**
 * Type-safe API request function
 * 
 * @template T - The expected response type (for documentation/type hints)
 * @param endpoint - API endpoint path
 * @param options - Fetch options (method, body, headers, etc.)
 * @returns Promise resolving to the Response object
 */
// eslint-disable-next-line @typescript-eslint/no-unused-vars
export const apiRequest = async <T = unknown>(
  endpoint: string,
  options: ApiRequestOptions = {}
): Promise<Response> => {
  // Use relative path by default to leverage the dev server proxy
  const API_BASE =
    (window as any).env?.REACT_APP_API_URL ||
    process.env.REACT_APP_API_URL ||
    '/api';

  const token = localStorage.getItem('token');

  const config: RequestInit = {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...(token && { Authorization: `Bearer ${token}` }),
      ...options.headers,
    },
  };

  // Ensure endpoint starts with / if not present (unless it's a full URL)
  const url = endpoint.startsWith('http')
    ? endpoint
    : `${API_BASE}${endpoint.startsWith('/') ? '' : '/'}${endpoint}`;

  const response = await fetch(url, config);
  return response;
};

/**
 * Type-safe API request that parses JSON response
 * 
 * @template T - The expected response type
 * @param endpoint - API endpoint path
 * @param options - Fetch options
 * @returns Promise resolving to parsed JSON data
 * @throws Error if response is not ok or JSON parsing fails
 */
export const apiRequestJson = async <T = unknown>(
  endpoint: string,
  options: ApiRequestOptions = {}
): Promise<T> => {
  const response = await apiRequest<T>(endpoint, options);

  if (!response.ok) {
    const errorText = await response.text().catch(() => 'Unknown error');
    throw new Error(`API request failed: ${response.status} ${errorText}`);
  }

  try {
    const data = await response.json();
    return data as T;
  } catch (error) {
    throw new Error(`Failed to parse JSON response: ${error}`);
  }
};

