// Use relative path by default to leverage the dev server proxy
const API_BASE = (window.env && window.env.REACT_APP_API_URL) || process.env.REACT_APP_API_URL || '/api';

export const apiRequest = async (endpoint, options = {}) => {
  const token = localStorage.getItem('token');
  
  const config = {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...(token && { Authorization: `Bearer ${token}` }),
      ...options.headers,
    },
  };

  // Ensure endpoint starts with / if not present (unless it's a full URL)
  const url = endpoint.startsWith('http') ? endpoint : `${API_BASE}${endpoint.startsWith('/') ? '' : '/'}${endpoint}`;

  const response = await fetch(url, config);
  return response;
};
