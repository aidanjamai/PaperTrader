# PaperTrader - System Architecture Documentation

## Table of Contents
1. [Deployment Architecture](#deployment-architecture)
2. [Application Architecture](#application-architecture)
3. [Database Schema](#database-schema)
4. [API Architecture](#api-architecture)
5. [Data Flow Diagrams](#data-flow-diagrams)
6. [Technology Stack](#technology-stack)

---

## Deployment Architecture

```mermaid
graph TB
    subgraph "Internet"
        User[👤 Users/Web Browsers]
    end

    subgraph "Docker Network"
        subgraph "Reverse Proxy Layer"
            Caddy[Caddy<br/>Port: 80/443<br/>Reverse Proxy<br/>& TLS Termination]
        end

        subgraph "Application Layer"
            Frontend[Frontend Container<br/>React Application<br/>Port: 3000]
            Backend[Backend Container<br/>Go HTTP Server<br/>Port: 8080<br/>Gorilla Mux Router]
        end

        subgraph "Data Layer"
            PostgreSQL[(PostgreSQL 15<br/>Port: 5432<br/>Transactional Data)]
            Redis[(Redis 7<br/>Port: 6379<br/>Cache & Rate Limiting)]
        end
    end

    subgraph "External Services"
        MarketStack[MarketStack API<br/>Stock Price Data<br/>External HTTP API]
    end

    User -->|HTTPS/HTTP| Caddy
    Caddy -->|HTTP| Frontend
    Caddy -->|HTTP| Backend
    Backend <-->|SQL Queries<br/>ACID Transactions| PostgreSQL
    Backend <-->|Cache Operations<br/>Rate Limiting| Redis
    Backend -->|HTTP API Calls| MarketStack

    style PostgreSQL fill:#336791,color:#fff
    style Redis fill:#DC382D,color:#fff
    style Caddy fill:#0CAD0C,color:#fff
    style Backend fill:#00ADD8,color:#fff
    style Frontend fill:#61DAFB,color:#333
```

---

## Application Architecture

```mermaid
graph TB
    subgraph "Client Layer"
        Browser[Web Browser<br/>React SPA]
    end

    subgraph "API Gateway Layer"
        Caddy[Caddy Reverse Proxy]
        Router[Gorilla Mux Router<br/>/api/*]
    end

    subgraph "Middleware Layer (Global, in order)"
        ReqLogger[RequestLogger<br/>Attaches request_id]
        Recover[Recover<br/>Panic handler]
        StripHdr[StripUserHeaders<br/>Strip forgable identity headers]
        CORS[CORS Middleware]
        OriginCheck[OriginCheck<br/>CSRF defence]
        SizeLimit[RequestSizeLimit]
        Timeout[RequestTimeout]
    end

    subgraph "Per-Route Middleware"
        JWT[JWT Auth Middleware<br/>Protected routes only]
        RateLimit[Rate Limit Middleware<br/>Redis sliding window<br/>Auth + Market + Watchlist POST]
    end

    subgraph "API Handlers Layer"
        AccountHandler[Account Handler<br/>Registration, Login, Profile<br/>Google OAuth, Email Verify]
        MarketHandler[Market Handler<br/>Stock Queries]
        InvestmentHandler[Investment Handler<br/>Buy/Sell, History]
        WatchlistHandler[Watchlist Handler<br/>List/Add/Remove]
    end

    subgraph "Business Logic Layer (Services)"
        AuthService[Auth Service<br/>User Management<br/>JWT Generation]
        GoogleOAuthService[Google OAuth Service<br/>ID Token Verification]
        EmailService[Email Service<br/>Resend API]
        MarketService[Market Service<br/>Stock Data<br/>Cache + DB-backed Series<br/>Empty-Range Negative Cache]
        InvestmentService[Investment Service<br/>Trade Execution<br/>ACID Transactions<br/>Idempotency Keys]
        WatchlistService[Watchlist Service]
        JWTService[JWT Service<br/>Token Validation]
    end

    subgraph "Data Access Layer (Stores)"
        UserStore[User Store<br/>CRUD Operations]
        TradeStore[Trade Store<br/>Append-only Trade Records]
        PortfolioStore[Portfolio Store<br/>Holdings Management]
        WatchlistStore[Watchlist Store]
        StockHistoryStore[Stock History Store<br/>Persisted EOD Closes<br/>Batched Upserts]
        StockCache[Stock Cache Interface<br/>Redis Implementation]
        HistoricalCache[Historical Cache Interface<br/>Redis + Empty-Range Memo]
    end

    subgraph "Persistence Layer"
        PostgreSQL[(PostgreSQL<br/>Users, Trades, Portfolio,<br/>Watchlist, Stock History)]
        Redis[(Redis<br/>Cache & Rate Limiting)]
    end

    Browser --> Caddy
    Caddy --> Router
    Router --> ReqLogger
    ReqLogger --> Recover
    Recover --> StripHdr
    StripHdr --> CORS
    CORS --> OriginCheck
    OriginCheck --> SizeLimit
    SizeLimit --> Timeout
    Timeout --> JWT
    JWT --> RateLimit
    RateLimit --> AccountHandler
    RateLimit --> MarketHandler
    RateLimit --> InvestmentHandler
    RateLimit --> WatchlistHandler

    AccountHandler --> AuthService
    AccountHandler --> GoogleOAuthService
    AuthService --> EmailService
    MarketHandler --> MarketService
    InvestmentHandler --> InvestmentService
    WatchlistHandler --> WatchlistService

    AuthService --> JWTService
    AuthService --> UserStore
    GoogleOAuthService --> UserStore
    GoogleOAuthService --> JWTService
    MarketService --> StockCache
    MarketService --> HistoricalCache
    MarketService --> StockHistoryStore
    InvestmentService --> MarketService
    InvestmentService --> UserStore
    InvestmentService --> TradeStore
    InvestmentService --> PortfolioStore
    WatchlistService --> WatchlistStore
    WatchlistService --> MarketService

    UserStore --> PostgreSQL
    TradeStore --> PostgreSQL
    PortfolioStore --> PostgreSQL
    WatchlistStore --> PostgreSQL
    StockHistoryStore --> PostgreSQL
    StockCache --> Redis
    HistoricalCache --> Redis

    style PostgreSQL fill:#336791,color:#fff
    style Redis fill:#DC382D,color:#fff
```

---

## Database Schema

```mermaid
erDiagram
    USERS ||--o{ TRADES : "has"
    USERS ||--o{ PORTFOLIO : "owns"
    USERS ||--o{ WATCHLIST : "watches"

    USERS {
        VARCHAR id PK "UUID"
        VARCHAR email UK "Unique"
        TEXT password "Bcrypt Hash, NULLABLE (Google users)"
        TIMESTAMP created_at
        NUMERIC balance "Default: 10000.00, CHECK >= 0"
        BOOLEAN email_verified "Default: false"
        VARCHAR verification_token "Email verification token"
        TIMESTAMP verification_token_expires
        VARCHAR google_id UK "Google OAuth subject id"
        VARCHAR created_via "email | google"
    }

    TRADES {
        VARCHAR id PK "UUID"
        VARCHAR user_id FK
        VARCHAR symbol
        VARCHAR action "BUY/SELL"
        INTEGER quantity
        NUMERIC price
        TIMESTAMPTZ executed_at "NOT NULL, default now()"
        VARCHAR status "PENDING/COMPLETED/FAILED"
        VARCHAR idempotency_key "Unique per user when set"
    }

    PORTFOLIO {
        VARCHAR id PK "UUID"
        VARCHAR user_id FK
        VARCHAR symbol
        INTEGER quantity
        NUMERIC avg_price "NUMERIC(20,8), widened for fractional pricing"
        TIMESTAMP created_at
        TIMESTAMP updated_at
        UNIQUE user_id_symbol "UNIQUE(user_id, symbol)"
    }

    WATCHLIST {
        VARCHAR id PK "UUID"
        VARCHAR user_id FK "REFERENCES users(id)"
        VARCHAR symbol
        TIMESTAMP created_at
        UNIQUE user_id_symbol "UNIQUE(user_id, symbol)"
    }

    STOCK_HISTORY {
        VARCHAR symbol PK "Composite PK"
        DATE trade_date PK "Composite PK"
        NUMERIC close "NUMERIC(20,8)"
        BIGINT volume "Daily share volume"
        TIMESTAMP fetched_at "Refreshed on conflict"
    }
```

> `STOCK_HISTORY` is a **shared cache table** — no relationship to `USERS`. Every user that views the same symbol's chart hits the same rows.

### Table Details

#### users
- **Purpose**: User accounts and authentication (email/password and Google OAuth)
- **Key Fields**:
  - `id`: UUID primary key
  - `email`: Unique email address
  - `password`: Bcrypt hashed password (cost factor 12). **Nullable** — null for Google-OAuth-only accounts.
  - `balance`: Starting balance $10,000.00 (CHECK constraint `balance >= 0`)
  - `email_verified`: Whether the user has confirmed their email (default `false`)
  - `verification_token` / `verification_token_expires`: Token issued for email verification flow
  - `google_id`: Unique Google subject id for OAuth-linked accounts
  - `created_via`: `email` or `google` — how the account was provisioned

#### trades
- **Purpose**: Append-only audit log of all buy/sell transactions
- **Key Fields**:
  - `id`: UUID primary key
  - `user_id`: Foreign key to users
  - `symbol`: Stock symbol (e.g., "AAPL")
  - `action`: "BUY" or "SELL"
  - `executed_at`: TIMESTAMPTZ, NOT NULL, defaults to `CURRENT_TIMESTAMP` (replaces the legacy `date VARCHAR` column)
  - `status`: Transaction status
  - `idempotency_key`: Optional client-supplied key; unique per `user_id` (partial unique index) so retried buy/sell requests don't double-execute
- **Triggers**: `BEFORE UPDATE` and `BEFORE DELETE` triggers raise an exception — the table is enforced as append-only at the database level.
- **Indexes**: `(user_id, executed_at DESC)` for trade history queries.

#### portfolio
- **Purpose**: Current holdings per user
- **Key Fields**:
  - `id`: UUID primary key
  - `user_id`: Foreign key to users
  - `symbol`: Stock symbol
  - `quantity`: Number of shares
  - `avg_price`: Weighted average purchase price, `NUMERIC(20,8)` (widened from `NUMERIC(15,2)` to support fractional pricing)
  - Unique constraint on (user_id, symbol)

#### watchlist
- **Purpose**: Per-user list of symbols the user is tracking (independent of holdings)
- **Key Fields**:
  - `id`: UUID primary key
  - `user_id`: Foreign key to `users(id)`
  - `symbol`: Stock symbol
  - `created_at`: When the symbol was added
  - Unique constraint on (user_id, symbol)
- **Indexes**: `idx_watchlist_user_id` on `user_id` for fast list lookups.

#### stock_history
- **Purpose**: Persisted EOD closes per symbol; backs the stock-detail price chart so repeat loads are served from Postgres rather than MarketStack.
- **Key Fields**:
  - `symbol` + `trade_date`: composite primary key (no surrogate id)
  - `close`: `NUMERIC(20,8)` — matches `portfolio.avg_price` precision
  - `volume`: `BIGINT` — high-volume tickers exceed `INTEGER`
  - `fetched_at`: refreshed on every conflicting upsert; useful for staleness audits
- **Indexes**: Primary key alone — every chart query is a `(symbol, trade_date BETWEEN ...)` range scan covered by the PK.
- **Notes**: Independent of users. Written by `MarketService.GetHistoricalSeries` via batched upserts (`upsertBatchSize = 1000` to stay under the Postgres parameter cap).

---

## API Architecture

```mermaid
graph LR
    subgraph "Health (Public)"
        Health[GET /health]
        ApiHealth[GET /api/health]
    end

    subgraph "Public Endpoints (Rate Limited)"
        Register[POST /api/account/register]
        Login[POST /api/account/login]
        GoogleLogin[POST /api/account/auth/google]
        VerifyEmail[GET /api/account/verify-email]
        ResendVerify[POST /api/account/resend-verification]
    end

    subgraph "Protected Endpoints - Account"
        Logout[POST /api/account/logout]
        Profile[GET /api/account/profile]
        AuthCheck[GET /api/account/auth]
        Balance[GET /api/account/balance]
    end

    subgraph "Protected Endpoints - Market (JWT + Rate Limit)"
        GetStock[GET /api/market/stock?symbol=AAPL]
        Historical[GET /api/market/stock/historical/daily?symbol=AAPL]
        Series[GET /api/market/stock/historical/series<br/>?symbol=AAPL&days=90<br/>DB-backed, Redis empty-range memo]
        BatchHistorical[GET /api/market/stock/historical/daily/batch<br/>JWT only — excluded from rate limit]
        PostStock[POST /api/market/stock]
        DeleteStock[DELETE /api/market/stock/symbol<br/>symbol in JSON body]
    end

    subgraph "Protected Endpoints - Investments"
        BuyStock[POST /api/investments/buy]
        SellStock[POST /api/investments/sell]
        GetStocks[GET /api/investments]
        TradeHistory[GET /api/investments/history]
    end

    subgraph "Protected Endpoints - Watchlist"
        ListWatch[GET /api/watchlist]
        AddWatch[POST /api/watchlist<br/>Rate Limited]
        RemoveWatch["DELETE /api/watchlist/{symbol}"]
    end

    Register --> RL
    Login --> RL
    GoogleLogin --> RL
    VerifyEmail --> RL
    ResendVerify --> RL
    Logout --> JWT
    Profile --> JWT
    AuthCheck --> JWT
    Balance --> JWT
    GetStock --> JWT
    Historical --> JWT
    Series --> JWT
    BatchHistorical --> JWT
    PostStock --> JWT
    DeleteStock --> JWT
    BuyStock --> JWT
    SellStock --> JWT
    GetStocks --> JWT
    TradeHistory --> JWT
    ListWatch --> JWT
    AddWatch --> JWT
    RemoveWatch --> JWT

    JWT[JWT Middleware<br/>Validates Token<br/>Sets X-User-ID Header]
    RL[Rate Limit Middleware<br/>Sliding window via Redis]
```

### API Endpoint Summary

| Method | Endpoint | Authentication | Description |
|--------|----------|----------------|-------------|
| GET | `/health` | None | Liveness/readiness probe (DB + Redis ping) |
| GET | `/api/health` | None | Same probe under the `/api` prefix for the frontend |
| POST | `/api/account/register` | Rate Limit | User registration |
| POST | `/api/account/login` | Rate Limit | User login (returns JWT cookie) |
| POST | `/api/account/auth/google` | Rate Limit | Sign in / sign up via Google ID token |
| GET | `/api/account/verify-email` | Rate Limit | Confirm email via token query param |
| POST | `/api/account/resend-verification` | Rate Limit | Resend verification email |
| POST | `/api/account/logout` | JWT | User logout |
| GET | `/api/account/profile` | JWT | Get user profile |
| GET | `/api/account/auth` | JWT | Check authentication status |
| GET | `/api/account/balance` | JWT | Get user balance |
| GET | `/api/market/stock` | JWT + Rate Limit | Get current stock price (`?symbol=AAPL`) |
| GET | `/api/market/stock/historical/daily` | JWT + Rate Limit | Get latest + previous close (single point) |
| GET | `/api/market/stock/historical/series` | JWT + Rate Limit | Get daily-close time series (`?symbol=&days=`); served from `stock_history` table with MarketStack fill-in for missing dates and a Redis negative cache for empty windows |
| GET | `/api/market/stock/historical/daily/batch` | JWT (no rate limit) | Batched historical fetch — excluded from rate limiting because it reduces total MarketStack calls |
| POST | `/api/market/stock` | JWT + Rate Limit | Create / refresh stock cache entry |
| DELETE | `/api/market/stock/symbol` | JWT + Rate Limit | Invalidate cached stock; **symbol provided in JSON request body** (path is literal `/stock/symbol`) |
| POST | `/api/investments/buy` | JWT | Buy stock shares |
| POST | `/api/investments/sell` | JWT | Sell stock shares |
| GET | `/api/investments` | JWT | Get user portfolio holdings |
| GET | `/api/investments/history` | JWT | Get user trade history (append-only ledger) |
| GET | `/api/watchlist` | JWT | List watched symbols |
| POST | `/api/watchlist` | JWT + Rate Limit | Add a symbol (rate-limited because it calls MarketStack) |
| DELETE | `/api/watchlist/{symbol}` | JWT | Remove a symbol from the watchlist |

> **Rate limiting notes**
> - The public auth endpoints (`/register`, `/login`, `/auth/google`, `/verify-email`, `/resend-verification`) are rate-limited even though they don't require a JWT, to deter brute-force and abuse.
> - All `/api/market/*` routes are rate-limited **except** `/stock/historical/daily/batch`, which is intentionally exempt (`backend/internal/api/market/routes.go:23-34`) because batching reduces upstream API calls.
> - On the watchlist, only `POST` is rate-limited (it calls MarketStack); `GET` and `DELETE` are DB-only and exempt.

---

## Data Flow Diagrams

### Authentication Flow

```mermaid
sequenceDiagram
    participant Client
    participant Caddy
    participant Backend
    participant AuthService
    participant UserStore
    participant PostgreSQL
    participant JWTService

    Client->>Caddy: POST /api/account/login
    Caddy->>Backend: Forward request
    Backend->>AuthService: Login(email, password)
    AuthService->>UserStore: GetUserByEmail(email)
    UserStore->>PostgreSQL: SELECT * FROM users WHERE email = $1
    PostgreSQL-->>UserStore: User data
    UserStore-->>AuthService: User object
    AuthService->>AuthService: Validate password (bcrypt)
    AuthService->>JWTService: GenerateToken(userID)
    JWTService-->>AuthService: JWT token
    AuthService-->>Backend: {token, user}
    Backend-->>Caddy: HTTP 200 + Set-Cookie
    Caddy-->>Client: JWT token in cookie
```

### Buy Stock Transaction Flow (ACID)

```mermaid
sequenceDiagram
    participant Client
    participant Backend
    participant InvestmentService
    participant MarketService
    participant Redis
    participant MarketStack
    participant PostgreSQL
    participant UserStore
    participant TradeStore
    participant PortfolioStore

    Client->>Backend: POST /api/investments/buy<br/>{symbol, quantity}
    Backend->>InvestmentService: BuyStock(userID, symbol, quantity)
    
    Note over InvestmentService: 1. Get Stock Price
    InvestmentService->>MarketService: GetStock(symbol)
    MarketService->>Redis: GET stock:symbol:date
    alt Cache Hit
        Redis-->>MarketService: Cached price
    else Cache Miss
        MarketService->>MarketStack: HTTP GET /v1/eod/latest
        MarketStack-->>MarketService: Stock data
        MarketService->>Redis: SET stock:symbol:date (TTL: 15min)
    end
    MarketService-->>InvestmentService: StockData{price}
    
    Note over InvestmentService,PostgreSQL: 2. ACID Transaction Begins
    InvestmentService->>PostgreSQL: BEGIN TRANSACTION
    
    Note over InvestmentService: 3. Validate Balance
    InvestmentService->>UserStore: GetUserByID(userID)
    UserStore->>PostgreSQL: SELECT balance FROM users WHERE id = $1
    PostgreSQL-->>UserStore: User data
    UserStore-->>InvestmentService: User{balance}
    
    Note over InvestmentService: 4. Deduct Balance
    InvestmentService->>UserStore: UpdateBalance(userID, newBalance)
    UserStore->>PostgreSQL: UPDATE users SET balance = $1 WHERE id = $2
    
    Note over InvestmentService: 5. Create Trade Record
    InvestmentService->>TradeStore: CreateTrade(trade)
    TradeStore->>PostgreSQL: INSERT INTO trades VALUES (...)
    
    Note over InvestmentService: 6. Update Portfolio
    InvestmentService->>PortfolioStore: UpdatePortfolioWithBuy(...)
    PortfolioStore->>PostgreSQL: INSERT ... ON CONFLICT DO UPDATE
    
    Note over InvestmentService,PostgreSQL: 7. Commit Transaction (All or Nothing)
    InvestmentService->>PostgreSQL: COMMIT
    PostgreSQL-->>InvestmentService: Success
    
    InvestmentService-->>Backend: UserStock holding
    Backend-->>Client: HTTP 200 + Portfolio data
```

### Market Data Caching Flow

```mermaid
sequenceDiagram
    participant Client
    participant MarketService
    participant Redis
    participant MarketStack

    Client->>MarketService: GET /api/market/stock?symbol=AAPL
    
    MarketService->>Redis: GET stock:AAPL:2024-01-15
    
    alt Cache Hit (TTL valid)
        Redis-->>MarketService: Cached StockData
        MarketService-->>Client: Return cached data
    else Cache Miss or Expired
        MarketService->>MarketStack: GET /v1/eod/latest?symbols=AAPL
        MarketStack-->>MarketService: Fresh StockData
        MarketService->>Redis: SET stock:AAPL:2024-01-15<br/>TTL: 15 minutes
        MarketService-->>Client: Return fresh data
    end
```

### Stock-History Series Fetch (DB-backed with gap fill)

The chart endpoint reads from Postgres first and only consults MarketStack
for dates that aren't already persisted. Empty MarketStack responses (weekend
gaps, holidays) are memoized in Redis so they don't burn quota on every load.

```mermaid
sequenceDiagram
    participant Client
    participant MarketService
    participant StockHistoryStore
    participant PostgreSQL
    participant Redis
    participant MarketStack

    Client->>MarketService: GET /api/market/stock/historical/series<br/>?symbol=AAPL&days=90

    MarketService->>StockHistoryStore: GetRange(AAPL, from, to)
    StockHistoryStore->>PostgreSQL: SELECT ... WHERE symbol=$1<br/>AND trade_date BETWEEN $2 AND $3
    PostgreSQL-->>StockHistoryStore: stored rows
    StockHistoryStore-->>MarketService: stored

    Note over MarketService: Compute backward gap [from, earliest-1]<br/>and forward gap [latest+1, to]

    loop for each non-empty gap
        MarketService->>Redis: GET historical-empty:AAPL:gapFrom:gapTo
        alt Memo present (recent empty result)
            Redis-->>MarketService: hit → skip API call
        else No memo
            MarketService->>MarketStack: GET /v1/eod?symbols=AAPL<br/>&date_from=&date_to=&offset=
            MarketStack-->>MarketService: page of EOD rows<br/>(paginated until short page)
            alt Rows returned
                MarketService->>StockHistoryStore: UpsertMany(rows)
                StockHistoryStore->>PostgreSQL: INSERT ... ON CONFLICT DO UPDATE<br/>(chunked at 1000 rows)
            else Zero rows (weekend/holiday)
                MarketService->>Redis: SET historical-empty:AAPL:gapFrom:gapTo<br/>TTL 6h
            end
        end
    end

    MarketService-->>Client: { symbol, from, to, points: [...] }
```

### Rate Limiting Flow

```mermaid
sequenceDiagram
    participant Client
    participant RateLimitMiddleware
    participant RedisRateLimiter
    participant Redis
    participant Backend

    Client->>RateLimitMiddleware: HTTP Request<br/>Header: X-User-ID
    RateLimitMiddleware->>RedisRateLimiter: CheckLimit(userID, ipAddress)
    
    Note over RedisRateLimiter,Redis: Sliding Window Algorithm
    RedisRateLimiter->>Redis: ZREMRANGEBYSCORE (remove old entries)
    RedisRateLimiter->>Redis: ZCARD (count current requests)
    RedisRateLimiter->>Redis: ZADD (add current request)
    RedisRateLimiter->>Redis: EXPIRE (set window TTL)
    
    Redis-->>RedisRateLimiter: Current count
    
    alt Under Limit
        RedisRateLimiter-->>RateLimitMiddleware: Allowed=true, Remaining=X
        RateLimitMiddleware->>Backend: Forward request
        Backend-->>Client: HTTP 200
    else Over Limit
        RedisRateLimiter-->>RateLimitMiddleware: Allowed=false, Remaining=0
        RateLimitMiddleware-->>Client: HTTP 429<br/>Rate Limit Exceeded<br/>Retry-After header
    end
```

---

## Technology Stack

### Frontend
- **Framework**: React 18 (TypeScript)
- **HTTP Client**: Fetch API
- **Routing**: React Router v6
- **Google OAuth**: `@react-oauth/google`
- **Build Tool**: Create React App (react-scripts) / npm
- **Type Checking**: TypeScript 5.9 (`tsc --noEmit`)

### Backend
- **Language**: Go 1.25.0
- **HTTP Framework**: Gorilla Mux
- **Database Driver**: lib/pq (PostgreSQL)
- **Migrations**: golang-migrate/migrate/v4 (embedded SQL migrations runner)
- **Cache/Queue**: go-redis/v9
- **Authentication**: JWT (golang-jwt/jwt/v5)
- **Google OAuth**: `google.golang.org/api` (ID token verification)
- **Email**: `resend-go/v2` (Resend API for verification emails)
- **Password Hashing**: golang.org/x/crypto/bcrypt
- **Decimal Math**: shopspring/decimal (for monetary values)

### Infrastructure
- **Reverse Proxy**: Caddy 2
- **Containerization**: Docker & Docker Compose
- **Database**: PostgreSQL 15-alpine
- **Cache**: Redis 7-alpine

### External Services
- **Market Data**: MarketStack API (REST)

### Security Features
- JWT-based authentication
- Bcrypt password hashing (cost factor 12)
- CORS middleware
- Rate limiting (sliding window, per-user and per-IP)
- SQL injection protection (parameterized queries)
- ACID transactions for financial operations

---

## Key Architectural Decisions

### 1. ACID Transactions for Financial Operations
- All buy/sell operations execute within a single PostgreSQL transaction
- Ensures atomicity: balance update + trade record + portfolio update happen together
- Eliminates distributed transaction issues from previous MongoDB architecture

### 2. Redis Caching Strategy
- **Stock Prices**: 15-minute TTL (balances freshness with API costs)
- **Historical Data**: 24-hour TTL (daily data changes once per day)
- **Rate Limiting**: Sliding window using Redis sorted sets

### 3. Materialized Portfolio Table
- Portfolio holdings stored as a materialized table in PostgreSQL
- Updated atomically with trades in the same transaction
- Eliminates need to recalculate from trade history

### 4. Service Layer Pattern
- Business logic separated from HTTP handlers
- Services are testable and reusable
- Clean separation of concerns: API → Service → Data

### 5. DBTX Interface
- Common interface for `*sql.DB` and `*sql.Tx`
- Enables transaction support across all data stores
- Allows services to manage transactions without tight coupling

---

## Performance Considerations

### Caching
- Stock prices cached for 15 minutes (reduces MarketStack API calls)
- Historical data cached for 24 hours (daily data)
- Redis in-memory storage provides sub-millisecond response times

### Database
- PostgreSQL connection pooling (10 max connections, 5 idle — sized to PostgreSQL `max_connections=50` on the e2-micro deployment)
- Indexed queries on `user_id` in portfolio and trades tables
- Unique constraint on `(user_id, symbol)` in portfolio for fast lookups

### Rate Limiting
- Per-user limit: 100 requests/hour
- Per-IP limit: 200 requests/hour
- Sliding window algorithm for accurate time-based limiting

---

## Scalability Notes

### Current Architecture
- Monolithic backend application
- Single PostgreSQL instance
- Single Redis instance
- Suitable for small to medium traffic

### Potential Improvements
- **Horizontal Scaling**: Add backend replicas behind load balancer
- **Database**: Read replicas for read-heavy workloads
- **Cache**: Redis Cluster for high availability
- **Message Queue**: Add queue system for async operations (e.g., email notifications)

---

## Deployment Notes

### Docker Compose Services
1. **caddy**: Reverse proxy and TLS termination
2. **frontend**: React application (built as static assets)
3. **backend**: Go HTTP server
4. **postgres**: PostgreSQL database with persistent volume
5. **redis**: Redis cache with persistent volume

### Health Checks
- PostgreSQL: `pg_isready` command
- Redis: `redis-cli ping` command
- Backend depends on database health before starting

### Environment Variables
- `DATABASE_URL`: PostgreSQL connection string
- `REDIS_URL`: Redis connection string
- `MARKETSTACK_API_KEY`: External API key
- `JWT_SECRET`: Secret for JWT signing
- `FRONTEND_URL`: CORS allowed origin

---

*Last Updated: 2026-05-03*
*Architecture Version: 2.2 — adds stock-history series endpoint backed by `stock_history` table with Redis empty-range negative cache, plus stock-detail page on the frontend (2.1 added watchlist, Google OAuth, email verification, idempotency keys on trades, and append-only trade triggers)*
