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

    subgraph "Middleware Layer"
        CORS[CORS Middleware]
        JWT[JWT Auth Middleware]
        RateLimit[Rate Limit Middleware<br/>Redis-based]
    end

    subgraph "API Handlers Layer"
        AccountHandler[Account Handler<br/>Registration, Login, Profile]
        MarketHandler[Market Handler<br/>Stock Queries]
        InvestmentHandler[Investment Handler<br/>Buy/Sell Operations]
    end

    subgraph "Business Logic Layer (Services)"
        AuthService[Auth Service<br/>User Management<br/>JWT Generation]
        MarketService[Market Service<br/>Stock Data<br/>Cache Management]
        InvestmentService[Investment Service<br/>Trade Execution<br/>ACID Transactions]
        JWTService[JWT Service<br/>Token Validation]
    end

    subgraph "Data Access Layer (Stores)"
        UserStore[User Store<br/>CRUD Operations]
        TradeStore[Trade Store<br/>Trade Records]
        PortfolioStore[Portfolio Store<br/>Holdings Management]
        StockCache[Stock Cache Interface<br/>Redis Implementation]
        HistoricalCache[Historical Cache Interface<br/>Redis Implementation]
    end

    subgraph "Persistence Layer"
        PostgreSQL[(PostgreSQL<br/>Users, Trades, Portfolio)]
        Redis[(Redis<br/>Cache & Rate Limiting)]
    end

    Browser --> Caddy
    Caddy --> Router
    Router --> CORS
    CORS --> JWT
    JWT --> RateLimit
    RateLimit --> AccountHandler
    RateLimit --> MarketHandler
    RateLimit --> InvestmentHandler

    AccountHandler --> AuthService
    MarketHandler --> MarketService
    InvestmentHandler --> InvestmentService

    AuthService --> JWTService
    AuthService --> UserStore
    MarketService --> StockCache
    MarketService --> HistoricalCache
    InvestmentService --> MarketService
    InvestmentService --> UserStore
    InvestmentService --> TradeStore
    InvestmentService --> PortfolioStore

    UserStore --> PostgreSQL
    TradeStore --> PostgreSQL
    PortfolioStore --> PostgreSQL
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
    
    USERS {
        VARCHAR id PK "UUID"
        VARCHAR email UK "Unique"
        TEXT password "Bcrypt Hash"
        TIMESTAMP created_at
        NUMERIC balance "Default: 10000.00"
    }
    
    TRADES {
        VARCHAR id PK "UUID"
        VARCHAR user_id FK
        VARCHAR symbol
        VARCHAR action "BUY/SELL"
        INTEGER quantity
        NUMERIC price
        VARCHAR date
        VARCHAR status "PENDING/COMPLETED/FAILED"
    }
    
    PORTFOLIO {
        VARCHAR id PK "UUID"
        VARCHAR user_id FK
        VARCHAR symbol
        INTEGER quantity
        NUMERIC avg_price
        TIMESTAMP created_at
        TIMESTAMP updated_at
        UNIQUE user_id_symbol "UNIQUE(user_id, symbol)"
    }

    USERS ||--o{ TRADES : creates
    USERS ||--o{ PORTFOLIO : holds
```

### Table Details

#### users
- **Purpose**: User accounts and authentication
- **Key Fields**:
  - `id`: UUID primary key
  - `email`: Unique email address
  - `password`: Bcrypt hashed password (cost factor 12)
  - `balance`: Starting balance $10,000.00

#### trades
- **Purpose**: Audit log of all buy/sell transactions
- **Key Fields**:
  - `id`: UUID primary key
  - `user_id`: Foreign key to users
  - `symbol`: Stock symbol (e.g., "AAPL")
  - `action`: "BUY" or "SELL"
  - `status`: Transaction status

#### portfolio
- **Purpose**: Current holdings per user
- **Key Fields**:
  - `id`: UUID primary key
  - `user_id`: Foreign key to users
  - `symbol`: Stock symbol
  - `quantity`: Number of shares
  - `avg_price`: Weighted average purchase price
  - Unique constraint on (user_id, symbol)

---

## API Architecture

```mermaid
graph LR
    subgraph "Public Endpoints"
        Register[POST /api/account/register]
        Login[POST /api/account/login]
    end

    subgraph "Protected Endpoints - Account"
        Logout[POST /api/account/logout]
        Profile[GET /api/account/profile]
        AuthCheck[GET /api/account/auth]
        Balance[GET /api/account/balance]
        UpdateBalance[POST /api/account/update-balance]
        AllUsers[GET /api/account/users]
    end

    subgraph "Protected Endpoints - Market"
        GetStock[GET /api/market/stock?symbol=AAPL]
        Historical[GET /api/market/stock/historical/daily?symbol=AAPL]
        PostStock[POST /api/market/stock]
        DeleteStock[DELETE /api/market/stock/symbol?symbol=AAPL]
    end

    subgraph "Protected Endpoints - Investments"
        BuyStock[POST /api/investments/buy]
        SellStock[POST /api/investments/sell]
        GetStocks[GET /api/investments]
    end

    Register --> JWT
    Login --> JWT
    Logout --> JWT
    Profile --> JWT
    AuthCheck --> JWT
    Balance --> JWT
    UpdateBalance --> JWT
    AllUsers --> JWT
    GetStock --> JWT
    Historical --> JWT
    PostStock --> JWT
    DeleteStock --> JWT
    BuyStock --> JWT
    SellStock --> JWT
    GetStocks --> JWT

    JWT[JWT Middleware<br/>Validates Token<br/>Sets X-User-ID Header]
```

### API Endpoint Summary

| Method | Endpoint | Authentication | Description |
|--------|----------|----------------|-------------|
| POST | `/api/account/register` | None | User registration |
| POST | `/api/account/login` | None | User login (returns JWT) |
| POST | `/api/account/logout` | JWT | User logout |
| GET | `/api/account/profile` | JWT | Get user profile |
| GET | `/api/account/auth` | JWT | Check authentication status |
| GET | `/api/account/balance` | JWT | Get user balance |
| POST | `/api/account/update-balance` | JWT | Update user balance |
| GET | `/api/account/users` | JWT | Get all users (admin) |
| GET | `/api/market/stock` | JWT + Rate Limit | Get current stock price |
| GET | `/api/market/stock/historical/daily` | JWT + Rate Limit | Get historical stock data |
| POST | `/api/market/stock` | JWT + Rate Limit | Create stock entry |
| DELETE | `/api/market/stock/symbol` | JWT + Rate Limit | Delete stock by symbol |
| POST | `/api/investments/buy` | JWT | Buy stock shares |
| POST | `/api/investments/sell` | JWT | Sell stock shares |
| GET | `/api/investments` | JWT | Get user portfolio holdings |

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
- **Framework**: React (JavaScript)
- **HTTP Client**: Fetch API / Axios
- **Routing**: React Router (implied)
- **Build Tool**: Create React App / npm

### Backend
- **Language**: Go 1.23
- **HTTP Framework**: Gorilla Mux
- **Database Driver**: lib/pq (PostgreSQL)
- **Cache/Queue**: go-redis/v9
- **Authentication**: JWT (golang-jwt/jwt/v5)
- **Password Hashing**: golang.org/x/crypto/bcrypt

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
- PostgreSQL connection pooling (25 max connections, 5 idle)
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

*Last Updated: After PostgreSQL Migration*  
*Architecture Version: 2.0 (PostgreSQL + Redis)*
