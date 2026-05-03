# PaperTrader API Documentation

Complete API reference for the PaperTrader backend service.

## Table of Contents

- [Base URL](#base-url)
- [Authentication](#authentication)
- [Endpoints](#endpoints)
  - [Health](#health-endpoints)
  - [Authentication](#authentication-endpoints)
  - [Trading](#trading-endpoints)
  - [Market Data](#market-data-endpoints)
  - [Watchlist](#watchlist-endpoints)

---

## Base URL

- **Development**: `http://localhost:8080/api`
- **Production**: `https://your-domain.com/api`

---

## Authentication

All protected endpoints require a JWT token. The token is set as an `HttpOnly`
cookie named `token` on successful login/register, and may also be supplied via
the standard `Authorization` header:

```
Authorization: Bearer <token>
```

The token is **only** delivered via cookie — login and register responses do
not include the token in the JSON body.

---

## Endpoints

### Health Endpoints

Two health endpoints are exposed (one for internal load-balancer probes, one
for the frontend). Both are unauthenticated and behave identically.

#### Internal Health

**GET** `/health`

#### API Health

**GET** `/api/health`

- **Response** (200 OK): plain-text body `OK`
- **Response** (503 Service Unavailable): plain-text body `DB_UNHEALTHY` or
  `REDIS_UNHEALTHY` when the corresponding dependency is unreachable.

---

### Authentication Endpoints

Base path: `/api/account`

The endpoints `/register`, `/login`, `/auth/google`, `/verify-email` and
`/resend-verification` are rate-limited (see [Rate Limiting](#rate-limiting)).

#### Register User

**POST** `/api/account/register`

Register a new user account. Sets the JWT as an `HttpOnly` cookie.

- **Request Body**:
  ```json
  {
    "email": "user@example.com",
    "password": "securepassword123"
  }
  ```

- **Response** (201 Created):
  ```json
  {
    "success": true,
    "message": "User registered successfully",
    "user": {
      "id": "uuid",
      "email": "user@example.com",
      "balance": 10000.00,
      "created_at": "2024-01-01T00:00:00Z",
      "email_verified": false,
      "created_via": "email"
    }
  }
  ```

  Note: the JWT is delivered as an `HttpOnly` cookie named `token`. It is
  **not** included in the response body.

- **Error Responses**:
  - `400 Bad Request` - Invalid input or email already exists
  - `429 Too Many Requests` - Rate limit exceeded
  - `500 Internal Server Error` - Server error

#### Login

**POST** `/api/account/login`

Authenticate user. Sets the JWT as an `HttpOnly` cookie.

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
      "created_at": "2024-01-01T00:00:00Z",
      "email_verified": true,
      "created_via": "email"
    }
  }
  ```

  Note: the JWT is delivered as an `HttpOnly` cookie named `token`. It is
  **not** included in the response body.

- **Error Responses**:
  - `401 Unauthorized` - Invalid credentials
  - `400 Bad Request` - Invalid input
  - `429 Too Many Requests` - Rate limit exceeded

#### Google OAuth Login

**POST** `/api/account/auth/google`

Exchange a Google ID token for a PaperTrader session. Creates the user on first
sign-in (with `created_via = "google"` and `email_verified = true`). Sets the
JWT as an `HttpOnly` cookie.

- **Request Body**:
  ```json
  {
    "token": "google-id-token-here"
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
      "created_at": "2024-01-01T00:00:00Z",
      "email_verified": true,
      "created_via": "google"
    }
  }
  ```

- **Error Responses**:
  - `400 Bad Request` - Missing or malformed `token`
  - `401 Unauthorized` - Google authentication failed
  - `429 Too Many Requests` - Rate limit exceeded

#### Verify Email

**GET** `/api/account/verify-email?token=<verification-token>`

Mark the user's email as verified using the one-time token sent in the
verification email.

- **Query Parameters**:
  - `token` (required) - the verification token from the email link

- **Response** (200 OK):
  ```json
  {
    "success": true,
    "message": "Email verified successfully"
  }
  ```

- **Error Responses**:
  - `400 Bad Request` - Missing, invalid, or expired token
  - `429 Too Many Requests` - Rate limit exceeded

#### Resend Verification Email

**POST** `/api/account/resend-verification`

Issue a new verification token for an unverified account.

- **Request Body**:
  ```json
  {
    "email": "user@example.com"
  }
  ```

- **Response** (200 OK):
  ```json
  {
    "success": true,
    "message": "Verification email sent"
  }
  ```

- **Error Responses**:
  - `400 Bad Request` - Invalid request body or service rejected the request
  - `429 Too Many Requests` - Rate limit exceeded

#### Logout

**POST** `/api/account/logout`

Logout user and clear the authentication cookie.

- **Headers**: Authorization required
- **Response** (200 OK):
  ```json
  {
    "success": true,
    "message": "Logout successful"
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
    "created_at": "2024-01-01T00:00:00Z",
    "email_verified": true,
    "created_via": "email"
  }
  ```

- **Error Responses**:
  - `401 Unauthorized` - Invalid or missing token

#### Check Authentication Status

**GET** `/api/account/auth`

Check if the current request is authenticated. The JWT middleware rejects
unauthenticated callers with `401 Unauthorized` before this handler runs; when
the handler does run, it always returns `200 OK`.

- **Headers**: Authorization required
- **Response** (200 OK):
  ```json
  {
    "success": true,
    "message": "Authentication check completed"
  }
  ```

- **Error Responses**:
  - `401 Unauthorized` - Not authenticated (returned by middleware)

#### Get Account Balance

**GET** `/api/account/balance`

Get current account balance.

- **Headers**: Authorization required
- **Response** (200 OK): a bare JSON number, e.g.
  ```json
  10000.00
  ```

- **Error Responses**:
  - `401 Unauthorized` - Not authenticated
  - `404 Not Found` - User not found

---

### Trading Endpoints

Base path: `/api/investments`

---

### Idempotency-Key Header

The `/api/investments/buy` and `/api/investments/sell` endpoints support an optional `Idempotency-Key` header to deduplicate retried requests.

- **Header name**: `Idempotency-Key`
- **Format**: Any printable ASCII characters (byte values 32–126), max 255 characters.
- **Scope**: Deduplication is per `(user_id, key)`. The same key may be reused across different users.
- **Behavior**: If a request with a given key has already been executed, the server returns the original response without executing the operation again. The original trade is returned regardless of whether the new request body differs (Stripe-style: the key, not the args, is the identity).
- **Concurrent retries**: If two requests with the same key arrive simultaneously, exactly one trade is committed; the other receives the same response.
- **Lifetime**: Keys are retained for the lifetime of the trade record. There is no TTL.
- **Errors**:
  - A blank/whitespace-only key returns `400 VALIDATION_ERROR` with message `"Idempotency-Key must not be blank or whitespace-only"`.
  - A key longer than 255 characters returns `400 VALIDATION_ERROR` with message `"Idempotency-Key must be 255 characters or fewer"`.
  - A key containing non-printable ASCII returns `400 VALIDATION_ERROR` with message `"Idempotency-Key must contain only printable ASCII characters"`.

```
Idempotency-Key: 550e8400-e29b-41d4-a716-446655440000
```

---

#### Buy Stock

**POST** `/api/investments/buy`

Purchase stock shares. This operation is atomic - balance deduction, trade creation, and portfolio update all succeed or all fail.

- **Headers**: Authorization required; `Idempotency-Key` optional (see above)
- **Request Body**:
  ```json
  {
    "symbol": "AAPL",
    "quantity": 10
  }
  ```

  The DTO also accepts a `userId` field for backwards compatibility, but it is
  ignored by the server — the user is always taken from the authenticated
  session (`X-User-ID` set by the JWT middleware).

- **Response** (200 OK):
  ```json
  {
    "id": "uuid",
    "userId": "uuid",
    "symbol": "AAPL",
    "quantity": 10,
    "avg_price": 150.00,
    "total": 1500.00,
    "profit": 0.00
  }
  ```

- **Error Responses**:
  - `400 Bad Request` - Invalid input (symbol, quantity, idempotency key)
  - `401 Unauthorized` - Not authenticated
  - `400 Bad Request` (`INSUFFICIENT_FUNDS`) - Insufficient funds
  - `404 Not Found` - Stock symbol not found
  - `500 Internal Server Error` - Transaction failed

- **Notes**:
  - Uses ACID transaction to ensure atomicity
  - Deducts balance, creates trade record, and updates portfolio in single transaction
  - Current stock price fetched from MarketStack API (cached in Redis)

#### Sell Stock

**POST** `/api/investments/sell`

Sell stock shares from portfolio. Returns cash to account balance.

- **Headers**: Authorization required; `Idempotency-Key` optional (see above)
- **Request Body**:
  ```json
  {
    "symbol": "AAPL",
    "quantity": 5
  }
  ```

  As with `/buy`, a `userId` field is accepted but ignored.

- **Response** (200 OK):
  ```json
  {
    "id": "uuid",
    "userId": "uuid",
    "symbol": "AAPL",
    "quantity": 5,
    "avg_price": 150.00,
    "total": 750.00,
    "profit": 10.00
  }
  ```

- **Error Responses**:
  - `400 Bad Request` - Invalid input (`INSUFFICIENT_STOCK` if not enough shares)
  - `401 Unauthorized` - Not authenticated
  - `404 Not Found` - Stock not in portfolio (`HOLDING_NOT_FOUND`)
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
      "userId": "uuid",
      "symbol": "AAPL",
      "quantity": 10,
      "avg_price": 150.00,
      "total": 1500.00,
      "profit": 20.00
    },
    {
      "id": "uuid",
      "userId": "uuid",
      "symbol": "GOOGL",
      "quantity": 5,
      "avg_price": 200.00,
      "total": 1000.00,
      "profit": 25.00
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

#### Get Trade History

**GET** `/api/investments/history`

Return a paginated, filterable list of the user's trades.

- **Headers**: Authorization required
- **Query Parameters** (all optional):
  - `limit` (integer, 1-200, default 50) - page size
  - `offset` (integer, ≥ 0, default 0) - rows to skip
  - `symbol` (string) - filter by symbol; validated like buy/sell
  - `action` (string) - filter by action; must be `BUY` or `SELL`

- **Response** (200 OK):
  ```json
  {
    "trades": [
      {
        "id": "uuid",
        "user_id": "uuid",
        "symbol": "AAPL",
        "action": "BUY",
        "quantity": 10,
        "price": 150.00,
        "total": 1500.00,
        "executed_at": "2024-01-01T12:34:56Z",
        "status": "COMPLETED",
        "idempotency_key": "550e8400-e29b-41d4-a716-446655440000"
      }
    ],
    "total": 142,
    "limit": 50,
    "offset": 0
  }
  ```

  `total` is the count of all trades matching the filter (independent of
  `limit`/`offset`) so the UI can render "showing 1-50 of 142".
  `idempotency_key` is omitted when the trade was created without one.

- **Error Responses**:
  - `400 Bad Request` (`VALIDATION_ERROR`) - bad `limit`, `offset`, `symbol`, or `action`
  - `401 Unauthorized` - Not authenticated

---

### Market Data Endpoints

Base path: `/api/market`

All market endpoints require a valid JWT (Authorization header or cookie). All
responses are wrapped in the standard market envelope:

```json
{
  "success": true,
  "message": "Stock data retrieved successfully",
  "data": { ... }
}
```

(error responses replace `data` with an optional `error` field — see [Error
Response Format](#error-response-format)).

#### Get Stock Price

**GET** `/api/market/stock?symbol=AAPL`

Get current stock price for a symbol.

- **Headers**: Authorization required
- **Query Parameters**:
  - `symbol` (required) - Stock symbol (e.g., "AAPL", "GOOGL")

- **Response** (200 OK):
  ```json
  {
    "success": true,
    "message": "Stock data retrieved successfully",
    "data": {
      "symbol": "AAPL",
      "date": "01/01/2024",
      "price": 150.00
    }
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

**GET** `/api/market/stock/historical/daily?symbol=AAPL`

Get historical daily data for a stock. The handler currently uses a fixed
internal date range and ignores any date query parameters.

- **Headers**: Authorization required
- **Query Parameters**:
  - `symbol` (required) - Stock symbol

- **Response** (200 OK):
  ```json
  {
    "success": true,
    "message": "Historical stock data retrieved successfully",
    "data": {
      "symbol": "AAPL",
      "date": "01/01/2024",
      "previous_price": 148.50,
      "price": 150.00,
      "volume": 50000000,
      "change": 1.50,
      "change_percentage": 1.01
    }
  }
  ```

- **Error Responses**:
  - `400 Bad Request` - Invalid symbol
  - `404 Not Found` (`INSUFFICIENT_DATA`) - Insufficient historical data
  - `429 Too Many Requests` - Rate limit exceeded
  - `500 Internal Server Error` - API error

- **Notes**:
  - Cached in Redis for 24 hours
  - Calculates price change and percentage change

#### Get Batch Historical Stock Data

**GET** `/api/market/stock/historical/daily/batch?symbols=AAPL,GOOGL,MSFT`

Fetch historical data for up to 15 symbols in a single MarketStack call. This
is the endpoint the dashboard uses to render multi-stock views and the
watchlist.

- **Headers**: Authorization required
- **Query Parameters**:
  - `symbols` (required) - comma- or space-separated list, max 15 symbols
- **Rate limiting**: this endpoint is **excluded** from per-user/IP rate
  limiting because it consolidates many lookups into one upstream call.

- **Response** (200 OK):
  ```json
  {
    "success": true,
    "message": "Historical data retrieved for 3 symbols",
    "data": {
      "AAPL": {
        "symbol": "AAPL",
        "date": "01/01/2024",
        "previous_price": 148.50,
        "price": 150.00,
        "volume": 50000000,
        "change": 1.50,
        "change_percentage": 1.01
      },
      "GOOGL": { "...": "..." },
      "MSFT":  { "...": "..." }
    }
  }
  ```

- **Error Responses**:
  - `400 Bad Request` - missing `symbols`, no symbols parsed, or more than 15
  - `500 Internal Server Error` - upstream API failure

#### Get Stock Price Series

**GET** `/api/market/stock/historical/series?symbol=AAPL&days=90`

Return a daily-close time series for one symbol over the requested window.
Reads from the local `stock_history` table whenever possible; only the gap
between the latest stored row and yesterday is fetched from MarketStack, so
repeat loads on the same symbol cost zero upstream API quota.

- **Headers**: Authorization required
- **Query Parameters**:
  - `symbol` (required) — Stock symbol
  - `days` (optional integer, default 90) — Lookback window in calendar days.
    Clamped to `[7, 365]`.

- **Response** (200 OK):
  ```json
  {
    "success": true,
    "message": "Historical series retrieved",
    "data": {
      "symbol": "AAPL",
      "from": "2024-10-04",
      "to": "2025-01-01",
      "points": [
        { "date": "2024-10-04", "close": 226.78 },
        { "date": "2024-10-07", "close": 221.69 }
      ]
    }
  }
  ```

- **Error Responses**:
  - `400 Bad Request` — Invalid symbol or non-positive `days`
  - `404 Not Found` (`INSUFFICIENT_DATA`) — No historical data for this symbol
  - `429 Too Many Requests` — Rate limit exceeded
  - `500 Internal Server Error` — Upstream API failure

- **Notes**:
  - Persists fetched closes to `stock_history(symbol, trade_date, close, volume)`
    so subsequent loads serve from Postgres.
  - The Redis 24-hour cache used by `GetHistoricalData` is **not** consulted by
    this endpoint — the DB is the primary cache.
  - When MarketStack returns zero rows for a gap window (typically a weekend or
    holiday), the `(symbol, from, to)` triple is memoized in Redis under
    `historical-empty:{symbol}:{from}:{to}` with a 6-hour TTL. Subsequent
    requests for the same gap skip the upstream call until the marker expires,
    so chart loads on Saturday and Sunday don't burn MarketStack quota.

#### Add Stock to Database

**POST** `/api/market/stock`

Add stock metadata to database (admin function).

- **Headers**: Authorization required
- **Request Body**:
  ```json
  {
    "symbol": "AAPL",
    "price": 150.00
  }
  ```

- **Response** (200 OK):
  ```json
  {
    "success": true,
    "message": "Stock data saved successfully",
    "data": {
      "symbol": "AAPL",
      "price": 150.00
    }
  }
  ```

- **Error Responses**:
  - `400 Bad Request` - Invalid input
  - `401 Unauthorized` - Not authenticated

#### Delete Stock from Database

**DELETE** `/api/market/stock/symbol`

Remove a stock from the database and invalidate its cache. Note the literal
path segment `/symbol` — this is **not** a path parameter. The symbol to
delete is supplied in the request body.

- **Headers**: Authorization required
- **Request Body**:
  ```json
  {
    "symbol": "AAPL"
  }
  ```

- **Response** (200 OK):
  ```json
  {
    "success": true,
    "message": "Stock cache invalidated successfully"
  }
  ```

- **Error Responses**:
  - `400 Bad Request` - Invalid JSON
  - `401 Unauthorized` - Not authenticated
  - `404 Not Found` - Stock not found

- **Notes**:
  - Invalidates Redis cache for the symbol
  - Does not affect existing trades or portfolio entries

---

### Watchlist Endpoints

Base path: `/api/watchlist`. All routes require a valid JWT.

#### List Watchlist

**GET** `/api/watchlist`

Return the user's watchlist enriched with current price data. Entries appear
even if the price lookup failed (`has_price: false`); the price/change fields
are zero in that case.

- **Headers**: Authorization required
- **Response** (200 OK):
  ```json
  {
    "items": [
      {
        "id": "uuid",
        "symbol": "AAPL",
        "created_at": "2024-01-01T12:34:56Z",
        "price": 150.00,
        "change": 1.50,
        "change_percentage": 1.01,
        "has_price": true
      }
    ]
  }
  ```

- **Error Responses**:
  - `401 Unauthorized` - Not authenticated

#### Add to Watchlist

**POST** `/api/watchlist`

Add a symbol to the user's watchlist. Validates the symbol against MarketStack
before inserting. **Rate-limited** because it always calls MarketStack.

- **Headers**: Authorization required
- **Request Body**:
  ```json
  {
    "symbol": "AAPL"
  }
  ```

- **Response** (201 Created):
  ```json
  {
    "id": "uuid",
    "symbol": "AAPL",
    "created_at": "2024-01-01T12:34:56Z",
    "price": 150.00,
    "change": 1.50,
    "change_percentage": 1.01,
    "has_price": true
  }
  ```

- **Error Responses**:
  - `400 Bad Request` (`INVALID_REQUEST`/`VALIDATION_ERROR`) - bad body or symbol
  - `401 Unauthorized` - Not authenticated
  - `404 Not Found` (`SYMBOL_NOT_FOUND`) - MarketStack has no data for this symbol
  - `409 Conflict` (`WATCHLIST_DUPLICATE`) - Symbol already in this user's watchlist
  - `429 Too Many Requests` - Rate limit exceeded

#### Remove from Watchlist

**DELETE** `/api/watchlist/{symbol}`

Remove a symbol from the user's watchlist. The symbol is taken from the URL
path.

- **Headers**: Authorization required
- **Response** (204 No Content): empty body
- **Error Responses**:
  - `401 Unauthorized` - Not authenticated
  - `404 Not Found` (`WATCHLIST_NOT_FOUND`) - The user is not watching this symbol

---

## Rate Limiting

Some endpoints are rate-limited via a Redis-backed sliding window. When Redis
is unavailable the server falls back to an in-memory limiter (state resets on
restart).

- **Per authenticated user**: 100 requests per hour
- **Per IP**: 200 requests per hour (also applied to authenticated requests)
- **Window**: 1 hour
- **Response on limit**: `429 Too Many Requests`

**Rate-limited endpoints**:
- All public auth routes: `/api/account/register`, `/api/account/login`,
  `/api/account/auth/google`, `/api/account/verify-email`,
  `/api/account/resend-verification`
- All `/api/market/*` routes **except** the batch historical endpoint
- `POST /api/watchlist`

`GET /api/market/stock/historical/daily/batch` is intentionally excluded
because it consolidates many MarketStack calls into one and therefore reduces
total upstream traffic.

---

## Error Response Format

The backend uses two error envelopes depending on the package handling the
request.

### Account / Investments / Watchlist

These use `util.WriteSafeError` / `util.WriteServiceError`, which include a
machine-readable `error_code`:

```json
{
  "success": false,
  "message": "Insufficient funds to complete this transaction",
  "error_code": "INSUFFICIENT_FUNDS"
}
```

Common error codes: `VALIDATION_ERROR`, `INVALID_REQUEST`, `EMAIL_EXISTS`,
`INVALID_CREDENTIALS`, `USER_NOT_FOUND`, `INSUFFICIENT_FUNDS`,
`INSUFFICIENT_STOCK`, `HOLDING_NOT_FOUND`, `INVALID_SYMBOL`,
`INSUFFICIENT_DATA`, `SYMBOL_NOT_FOUND`, `WATCHLIST_DUPLICATE`,
`WATCHLIST_NOT_FOUND`, `AUTH_REQUIRED`, `TOKEN_ERROR`, `INTERNAL_ERROR`.

### Market

The market package uses its own `MarketResponse`/`ErrorResponse` envelope
(no `error_code`):

```json
{
  "success": false,
  "message": "Invalid stock symbol",
  "error": "optional internal detail"
}
```

`error` is omitted when there is no extra detail to report.

## Status Codes

- `200 OK` - Request successful
- `201 Created` - Resource created (register, watchlist add)
- `204 No Content` - Resource deleted (watchlist remove)
- `400 Bad Request` - Invalid input or request format
- `401 Unauthorized` - Authentication required or invalid token
- `404 Not Found` - Resource not found
- `409 Conflict` - Duplicate resource (e.g. watchlist symbol already present)
- `429 Too Many Requests` - Rate limit exceeded
- `500 Internal Server Error` - Server error
- `503 Service Unavailable` - Health check found a failing dependency

---

## Data Types

### User Object
```typescript
{
  id: string;             // UUID
  email: string;          // Email address
  balance: number;        // Account balance (2 decimal places)
  created_at: string;     // ISO 8601 timestamp
  email_verified: boolean;
  created_via: string;    // "email" or "google"
}
```

### Portfolio Entry (UserStock)
```typescript
{
  id: string;        // UUID
  userId: string;    // UUID
  symbol: string;    // Stock symbol (1-10 chars)
  quantity: number;  // Number of shares (integer)
  avg_price: number; // Average purchase price (2 decimals)
  total: number;     // Current market value of the holding
  profit: number;    // Unrealized P/L (2 decimals)
}
```

### Trade
```typescript
{
  id: string;
  user_id: string;
  symbol: string;
  action: "BUY" | "SELL";
  quantity: number;
  price: number;
  total: number;
  executed_at: string;        // ISO 8601 timestamp
  status: "PENDING" | "COMPLETED" | "FAILED";
  idempotency_key?: string;   // omitted when not supplied
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

### Watchlist Entry
```typescript
{
  id: string;                // UUID
  symbol: string;
  created_at: string;        // ISO 8601 timestamp
  price: number;             // 0 when has_price is false
  change: number;            // 0 when has_price is false
  change_percentage: number; // 0 when has_price is false
  has_price: boolean;        // false when MarketStack lookup failed
}
```

---

## Notes

- All monetary values are rounded to 2 decimal places
- Stock symbols are case-insensitive (automatically uppercased)
- Stock data uses **MM/DD/YYYY** for the `date` display field; everything
  else (`created_at`, `executed_at`, etc.) uses ISO 8601 (Go `time.Time` JSON
  default)
- Transactions are ACID-compliant (atomic operations)
- Cache TTLs: Stock prices (15 min), Historical data (24 hr)
- MarketStack API integration requires a valid API key
