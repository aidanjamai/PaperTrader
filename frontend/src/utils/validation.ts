/**
 * Form validation utilities with TypeScript types
 */

/**
 * Email validation result
 */
export interface EmailValidationResult {
  isValid: boolean;
  error?: string;
}

/**
 * Password validation result
 */
export interface PasswordValidationResult {
  isValid: boolean;
  errors: string[];
}

/**
 * Stock symbol validation result
 */
export interface SymbolValidationResult {
  isValid: boolean;
  error?: string;
}

/**
 * Validates email format
 */
export const validateEmail = (email: string): EmailValidationResult => {
  if (!email || email.trim() === '') {
    return {
      isValid: false,
      error: 'Email is required'
    };
  }

  const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
  if (!emailRegex.test(email)) {
    return {
      isValid: false,
      error: 'Invalid email format'
    };
  }

  return { isValid: true };
};

/**
 * Validates password strength
 */
export const validatePassword = (password: string, minLength: number = 8): PasswordValidationResult => {
  const errors: string[] = [];

  if (!password || password.length === 0) {
    errors.push('Password is required');
    return { isValid: false, errors };
  }

  if (password.length < minLength) {
    errors.push(`Password must be at least ${minLength} characters long`);
  }

  if (!/[A-Z]/.test(password)) {
    errors.push('Password must contain at least one uppercase letter');
  }

  if (!/[a-z]/.test(password)) {
    errors.push('Password must contain at least one lowercase letter');
  }

  if (!/[0-9]/.test(password)) {
    errors.push('Password must contain at least one number');
  }

  // Check for special characters (matches backend validation)
  const specialCharRegex = /[!@#$%^&*()\-_+=[\]{}|\\:;"'<>,.?/~`]/;
  if (!specialCharRegex.test(password)) {
    errors.push('Password must contain at least one special character (!@#$%^&*() etc.)');
  }

  return {
    isValid: errors.length === 0,
    errors
  };
};

/**
 * Validates stock symbol format
 */
export const validateStockSymbol = (symbol: string): SymbolValidationResult => {
  if (!symbol || symbol.trim() === '') {
    return {
      isValid: false,
      error: 'Stock symbol is required'
    };
  }

  const trimmedSymbol = symbol.trim().toUpperCase();
  
  // Stock symbols are typically 1-5 characters, alphanumeric
  const symbolRegex = /^[A-Z0-9]{1,10}$/;
  if (!symbolRegex.test(trimmedSymbol)) {
    return {
      isValid: false,
      error: 'Invalid stock symbol format. Use 1-10 uppercase letters/numbers.'
    };
  }

  return { isValid: true };
};

/**
 * Validates quantity (positive integer)
 */
export const validateQuantity = (quantity: string | number): { isValid: boolean; value?: number; error?: string } => {
  const num = typeof quantity === 'string' ? parseInt(quantity, 10) : quantity;

  if (isNaN(num)) {
    return {
      isValid: false,
      error: 'Quantity must be a valid number'
    };
  }

  if (num <= 0) {
    return {
      isValid: false,
      error: 'Quantity must be a positive whole number'
    };
  }

  if (!Number.isInteger(num)) {
    return {
      isValid: false,
      error: 'Quantity must be a whole number (no decimals)'
    };
  }

  return {
    isValid: true,
    value: num
  };
};

/**
 * Validates price (positive number)
 */
export const validatePrice = (price: string | number): { isValid: boolean; value?: number; error?: string } => {
  const num = typeof price === 'string' ? parseFloat(price) : price;

  if (isNaN(num)) {
    return {
      isValid: false,
      error: 'Price must be a valid number'
    };
  }

  if (num < 0) {
    return {
      isValid: false,
      error: 'Price cannot be negative'
    };
  }

  return {
    isValid: true,
    value: num
  };
};

