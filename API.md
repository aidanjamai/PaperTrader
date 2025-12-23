# PaperTrader API Documentation

Complete API reference for the PaperTrader backend service.

## Table of Contents

- [Base URL](#base-url)
- [Authentication](#authentication)
- [Endpoints](#endpoints)
  - [Authentication](#authentication-endpoints)
  - [Trading](#trading-endpoints)
  - [Market Data](#market-data-endpoints)

---

## Base URL

- **Development**: `http://localhost:8080/api`
- **Production**: `https://your-domain.com/api`

---

## Authentication

All protected endpoints require a JWT token in the `Authorization` header:

```
Authorization: Bearer <token>
```

Tokens are automatically set as cookies on login/register and can be read from cookies or headers.

---

## Endpoints

### Authentication Endpoints

Base path: `/api/account`

#### Register User

**POST** `/api/account/register`

Register a new user account.

- **Request Body**:
  ```json
  {
    "email": "user@example.com",
    "password": "securepassword123"
  }
  ```

- **Response** (200 OK):
  ```json
  {
    "success": true,
    "message": "Registration successful",
    "user": {
      "id": "uuid",
      "email": "user@example.com",
      "balance": 10000.00,
      "created_at": "2024-01-01T00:00:00Z"
    },
    "token": "jwt_token_here"
  }
  ```

- **Error Responses**:
  - `400 Bad Request` - Invalid input or email already exists
  - `500 Internal Server Error` - Server error

#### Login

**POST** `/api/account/login`

Authenticate user and receive JWT token.

- **Request Body**:
  ```json
  {
    "email": "user@example.com",
    "password": "securepassword123"
  }
  ```

- **Response** (200 OK):
  ```json
  {
    "success": true,
    "message": "Login successful",
    "user": {
      "id": "uuid",
      "email": "user@example.com",
      "balance": 10000.00,
      "created_at": "2024-01-01T00:00:00Z"
    },
    "token": "jwt_token_here"
  }
  ```

- **Error Responses**:
  - `401 Unauthorized` - Invalid credentials
  - `400 Bad Request` - Invalid input

#### Logout

**POST** `/api/account/logout`

Logout user and clear authentication token.

- **Headers**: Authorization required
- **Response** (200 OK):
  ```json
  {
    "success": true,
    "message": "Logged out successfully"
  }
  ```

#### Get User Profile

**GET** `/api/account/profile`

Get current authenticated user's profile information.

- **Headers**: Authorization required
- **Response** (200 OK):
  ```json
  {
    "id": "uuid",
    "email": "user@example.com",
    "balance": 10000.00,
    "created_at": "2024-01-01T00:00:00Z"
  }
  ```

- **Error Responses**:
  - `401 Unauthorized` - Invalid or missing token

#### Check Authentication Status

**GET** `/api/account/auth`

Check if the current request is authenticated.

- **Headers**: Authorization required
- **Response** (200 OK):
  ```json
  {
    "success": true,
    "message": "Authenticated"
  }
  ```

- **Error Responses**:
  - `401 Unauthorized` - Not authenticated

#### Get Account Balance

**GET** `/api/account/balance`

Get current account balance.

- **Headers**: Authorization required
- **Response** (200 OK):
  ```json
  {
    "balance": 10000.00
  }
  ```

#### Update Account Balance

**PUT** `/api/account/balance`

Update account balance (admin/manual adjustment).

- **Headers**: Authorization required
- **Request Body**:
  ```json
  {
    "balance": 15000.00
  }
  ```

- **Response** (200 OK):
  ```json
  {
    "success": true,
    "message": "Balance updated successfully",
    "balance": 15000.00
  }
  ```

- **Error Responses**:
  - `400 Bad Request` - Invalid balance value
  - `401 Unauthorized` - Not authenticated

---

### Trading Endpoints

Base path: `/api/investments`

#### Buy Stock

**POST** `/api/investments/buy`

Purchase stock shares. This operation is atomic - balance deduction, trade creation, and portfolio update all succeed or all fail.

- **Headers**: Authorization required
- **Request Body**:
  ```json
  {
    "symbol": "AAPL",
    "quantity": 10
  }
  ```

- **Response** (200 OK):
  ```json
  {
    "id": "uuid",
    "user_id": "uuid",
    "symbol": "AAPL",
    "quantity": 10,
    "avg_price": 150.00,
    "total": 1500.00,
    "current_stock_price": 152.00,
    "created_at": "2024-01-01T00:00:00Z",
    "updated_at": "2024-01-01T00:00:00Z"
  }
  ```

- **Error Responses**:
  - `400 Bad Request` - Invalid input (symbol, quantity)
  - `401 Unauthorized` - Not authenticated
  - `402 Payment Required` - Insufficient funds
  - `404 Not Found` - Stock symbol not found
  - `500 Internal Server Error` - Transaction failed

- **Notes**:
  - Uses ACID transaction to ensure atomicity
  - Deducts balance, creates trade record, and updates portfolio in single transaction
  - Current stock price fetched from MarketStack API (cached in Redis)

#### Sell Stock

**POST** `/api/investments/sell`

Sell stock shares from portfolio. Returns cash to account balance.

- **Headers**: Authorization required
- **Request Body**:
  ```json
  {
    "symbol": "AAPL",
    "quantity": 5
  }
  ```

- **Response** (200 OK):
  ```json
  {
    "id": "uuid",
    "user_id": "uuid",
    "symbol": "AAPL",
    "quantity": 5,
    "avg_price": 150.00,
    "total": 750.00,
    "current_stock_price": 152.00,
    "created_at": "2024-01-01T00:00:00Z",
    "updated_at": "2024-01-01T00:00:00Z"
  }
  ```

- **Error Responses**:
  - `400 Bad Request` - Invalid input or insufficient shares
  - `401 Unauthorized` - Not authenticated
  - `404 Not Found` - Stock not in portfolio
  - `500 Internal Server Error` - Transaction failed

- **Notes**:
  - Uses ACID transaction
  - Validates sufficient shares before selling
  - Updates portfolio or removes entry if quantity reaches zero

#### Get Portfolio

**GET** `/api/investments`

Get user's complete portfolio (all stock holdings).

- **Headers**: Authorization required
- **Response** (200 OK):
  ```json
  [
    {
      "id": "uuid",
      "user_id": "uuid",
      "symbol": "AAPL",
      "quantity": 10,
      "avg_price": 150.00,
      "total": 1500.00,
      "current_stock_price": 152.00,
      "created_at": "2024-01-01T00:00:00Z",
      "updated_at": "2024-01-01T00:00:00Z"
    },
    {
      "id": "uuid",
      "user_id": "uuid",
      "symbol": "GOOGL",
      "quantity": 5,
      "avg_price": 200.00,
      "total": 1000.00,
      "current_stock_price": 205.00,
      "created_at": "2024-01-01T00:00:00Z",
      "updated_at": "2024-01-01T00:00:00Z"
    }
  ]
  ```

- **Response** (200 OK, empty portfolio):
  ```json
  []
  ```

- **Error Responses**:
  - `401 Unauthorized` - Not authenticated

- **Notes**:
  - Returns empty array if user has no holdings
  - Current stock prices are fetched from MarketStack API (cached in Redis)
  - Prices are rounded to 2 decimal places

---

### Market Data Endpoints

Base path: `/api/market`

#### Get Stock Price

**GET** `/api/market/stock/:symbol`

Get current stock price for a symbol.

- **Headers**: Authorization optional (rate limiting applies)
- **URL Parameters**:
  - `symbol` (path) - Stock symbol (e.g., "AAPL", "GOOGL")

- **Response** (200 OK):
  ```json
  {
    "symbol": "AAPL",
    "date": "01/01/2024",
    "price": 150.00
  }
  ```

- **Error Responses**:
  - `400 Bad Request` - Invalid symbol format
  - `404 Not Found` - Symbol not found in MarketStack
  - `429 Too Many Requests` - Rate limit exceeded
  - `500 Internal Server Error` - API error or timeout

- **Notes**:
  - Cached in Redis for 15 minutes
  - Fetches from MarketStack API on cache miss
  - Symbol validation: 1-10 uppercase letters/numbers

#### Get Historical Stock Data

**GET** `/api/market/stock/:symbol/historical/daily`

Get historical daily data for a stock (last 2 days by default).

- **Headers**: Authorization optional
- **URL Parameters**:
  - `symbol` (path) - Stock symbol
- **Query Parameters** (optional):
  - `start_date` - Start date (YYYY-MM-DD format)
  - `end_date` - End date (YYYY-MM-DD format)

- **Response** (200 OK):
  ```json
  {
    "symbol": "AAPL",
    "date": "01/01/2024",
    "previous_price": 148.50,
    "price": 150.00,
    "volume": 50000000,
    "change": 1.50,
    "change_percentage": 1.01
  }
  ```

- **Error Responses**:
  - `400 Bad Request` - Invalid symbol or date format
  - `404 Not Found` - Insufficient historical data
  - `429 Too Many Requests` - Rate limit exceeded
  - `500 Internal Server Error` - API error

- **Notes**:
  - Defaults to last 2 days if dates not specified
  - Cached in Redis for 24 hours
  - Calculates price change and percentage change

#### Add Stock to Database

**POST** `/api/market/stock`

Add stock metadata to database (admin function).

- **Headers**: Authorization required
- **Request Body**:
  ```json
  {
    "symbol": "AAPL",
    "name": "Apple Inc."
  }
  ```

- **Response** (200 OK):
  ```json
  {
    "success": true,
    "message": "Stock added successfully",
    "stock": {
      "symbol": "AAPL",
      "name": "Apple Inc.",
      "created_at": "2024-01-01T00:00:00Z"
    }
  }
  ```

- **Error Responses**:
  - `400 Bad Request` - Invalid input or symbol already exists
  - `401 Unauthorized` - Not authenticated

#### Delete Stock from Database

**DELETE** `/api/market/stock/:symbol`

Remove stock from database and invalidate cache (admin function).

- **Headers**: Authorization required
- **URL Parameters**:
  - `symbol` (path) - Stock symbol to delete

- **Response** (200 OK):
  ```json
  {
    "success": true,
    "message": "Stock deleted successfully"
  }
  ```

- **Error Responses**:
  - `404 Not Found` - Stock not found
  - `401 Unauthorized` - Not authenticated

- **Notes**:
  - Invalidates Redis cache for the symbol
  - Does not affect existing trades or portfolio entries

---

## Rate Limiting

Market data endpoints are rate-limited to prevent API abuse:

- **Per User**: Limited requests per authenticated user
- **Per IP**: Limited requests per IP address (for unauthenticated requests)
- **Implementation**: Redis-based sliding window rate limiting
- **Response**: `429 Too Many Requests` when limit exceeded

Rate limits are configured in the backend service layer.

---

## Error Response Format

All error responses follow this format:

```json
{
  "success": false,
  "message": "Error description here"
}
```

Some errors may include additional fields:
```json
{
  "success": false,
  "message": "Error description",
  "error": "Detailed error information"
}
```

## Status Codes

- `200 OK` - Request successful
- `400 Bad Request` - Invalid input or request format
- `401 Unauthorized` - Authentication required or invalid token
- `402 Payment Required` - Insufficient funds (buy operation)
- `404 Not Found` - Resource not found
- `429 Too Many Requests` - Rate limit exceeded
- `500 Internal Server Error` - Server error

---

## Data Types

### User Object
```typescript
{
  id: string;           // UUID
  email: string;        // Email address
  balance: number;      // Account balance (2 decimal places)
  created_at: string;   // ISO 8601 timestamp
}
```

### Portfolio Entry
```typescript
{
  id: string;                    // UUID
  user_id: string;               // UUID
  symbol: string;                // Stock symbol (1-10 chars)
  quantity: number;              // Number of shares (integer)
  avg_price: number;             // Average purchase price (2 decimals)
  total: number;                 // Total value (quantity * avg_price)
  current_stock_price: number;   // Current market price (2 decimals)
  created_at: string;            // ISO 8601 timestamp
  updated_at: string;            // ISO 8601 timestamp
}
```

### Stock Data
```typescript
{
  symbol: string;  // Stock symbol
  date: string;    // Date in MM/DD/YYYY format
  price: number;   // Stock price (2 decimal places)
}
```

### Historical Data
```typescript
{
  symbol: string;            // Stock symbol
  date: string;              // Date in MM/DD/YYYY format
  previous_price: number;    // Previous day's price
  price: number;             // Current price
  volume: number;            // Trading volume (integer)
  change: number;            // Price change (2 decimals)
  change_percentage: number; // Percentage change (2 decimals)
}
```

---

## Notes

- All monetary values are rounded to 2 decimal places
- Stock symbols are case-insensitive (automatically uppercased)
- Dates use MM/DD/YYYY format for display, YYYY-MM-DD for API queries
- All timestamps use ISO 8601 format (UTC)
- Transactions are ACID-compliant (atomic operations)
- Cache TTLs: Stock prices (15 min), Historical data (24 hr)
- MarketStack API integration requires valid API key
