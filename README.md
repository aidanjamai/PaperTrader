# PaperTrader

A full-stack paper trading platform that allows users to practice trading stocks without risking real money. Built with Go (Golang) backend and React (TypeScript) frontend, featuring real-time market data, portfolio management, and comprehensive trading tools.

## Table of Contents

- [Overview](#overview)
- [Tech Stack](#tech-stack)
- [Architecture](#architecture)
- [Features](#features)
- [Project Structure](#project-structure)
- [Setup & Installation](#setup--installation)
- [API Documentation](#api-documentation)
- [Database Schema](#database-schema)
- [Configuration](#configuration)
- [Development](#development)
- [Deployment](#deployment)
- [Security](#security)

---

## Overview

PaperTrader is a simulated stock trading platform that provides:

- **Secure User Authentication** - JWT-based authentication with secure password hashing
- **Real-Time Market Data** - Integration with MarketStack API for current stock prices and historical data
- **Portfolio Management** - Track holdings, calculate portfolio value, and manage investments
- **Trading Operations** - Buy and sell stocks with ACID transaction guarantees
- **Financial Calculators** - Portfolio calculator and compound interest calculator tools
- **Performance Tracking** - Monitor gains/losses and portfolio performance over time

---

## Tech Stack

### Backend

- **Language**: Go 1.23.0
- **Web Framework**: Gorilla Mux (HTTP router and URL matcher)
- **Database**: PostgreSQL 15 (ACID-compliant relational database)
- **Cache & Rate Limiting**: Redis 7 (in-memory data store)
- **Authentication**: JWT (JSON Web Tokens) with golang-jwt/jwt/v5
- **Password Hashing**: bcrypt via golang.org/x/crypto
- **External API**: MarketStack API (stock market data)
- **Environment Management**: godotenv

**Key Backend Libraries**:
- `github.com/gorilla/mux` - HTTP router
- `github.com/lib/pq` - PostgreSQL driver
- `github.com/redis/go-redis/v9` - Redis client
- `github.com/golang-jwt/jwt/v5` - JWT implementation
- `github.com/google/uuid` - UUID generation
- `golang.org/x/crypto` - Cryptographic functions

### Frontend

- **Framework**: React 18.2.0
- **Language**: TypeScript 5.9.3
- **Routing**: React Router DOM 6.3.0
- **Build Tool**: Create React App (react-scripts 5.0.1)
- **UI Framework**: Bootstrap 5.3.8
- **Component Library**: React Bootstrap 2.10.10
- **HTTP Client**: Fetch API with type-safe wrappers

**Key Frontend Libraries**:
- `react` & `react-dom` - UI framework
- `react-router-dom` - Client-side routing
- `typescript` - Type safety
- `bootstrap` & `react-bootstrap` - UI styling and components

### Infrastructure

- **Reverse Proxy**: Caddy 2 (HTTP/2, HTTPS, automatic SSL)
- **Containerization**: Docker & Docker Compose
- **Database**: PostgreSQL 15 Alpine
- **Cache**: Redis 7 Alpine

---

## Architecture

### System Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     Client (Browser)                        │
└───────────────────────┬─────────────────────────────────────┘
                        │ HTTPS/HTTP
                        ▼
┌─────────────────────────────────────────────────────────────┐
│              Caddy (Reverse Proxy & TLS)                    │
│                  Ports: 80 (HTTP), 443 (HTTPS)              │
└──────┬────────────────────────────────────────────┬─────────┘
       │                                             │
       │ /api/*                                      │ /*
       ▼                                             ▼
┌──────────────────┐                      ┌──────────────────┐
│  Backend (Go)    │                      │ Frontend (React) │
│  Port: 8080      │                      │  Port: 3000/80   │
│  Gorilla Mux     │                      │  TypeScript SPA  │
└──────┬───────────┘                      └──────────────────┘
       │
       ├──► PostgreSQL (Port 5432)
       │    └─ Users, Trades, Portfolio
       │
       ├──► Redis (Port 6379)
       │    └─ Stock Price Cache (15min TTL)
       │    └─ Historical Data Cache (24hr TTL)
       │    └─ Rate Limiting
       │
       └──► MarketStack API (External)
            └─ Real-time Stock Data
```

### Application Architecture

**Backend Layers**:
1. **API Handlers** (`internal/api/`) - HTTP request handlers
2. **Middleware** (`internal/api/middleware/`) - CORS, JWT auth, rate limiting
3. **Services** (`internal/service/`) - Business logic layer
4. **Data Stores** (`internal/data/`) - Database access layer
5. **Configuration** (`internal/config/`) - Config management

**Frontend Layers**:
1. **Components** (`src/components/`) - React UI components (organized by feature)
2. **Services** (`src/services/`) - API client services
3. **Hooks** (`src/hooks/`) - Custom React hooks
4. **Types** (`src/types/`) - TypeScript type definitions
5. **Utils** (`src/utils/`) - Utility functions (validation, etc.)

---

## Features

### Authentication & User Management

- **Secure Registration** - Email-based registration with password validation
- **JWT Authentication** - Stateless authentication using JSON Web Tokens
- **Password Security** - bcrypt hashing with salt (cost factor 10)
- **Session Management** - Token-based sessions with automatic expiration
- **User Profiles** - View account information, balance, and member since date
- **Balance Management** - Track and update account balance for paper trading

### Trading Features

- **Buy Stocks** - Purchase stocks with real-time pricing from MarketStack API
- **Sell Stocks** - Sell holdings with automatic portfolio updates
- **ACID Transactions** - All trading operations use PostgreSQL transactions ensuring:
  - Balance deduction/credit
  - Trade record creation
  - Portfolio update
  - All succeed or all fail (atomicity)
- **Portfolio Tracking** - Real-time portfolio value calculations
- **Holdings Display** - View all stock positions with current prices and values

### Market Data

- **Real-Time Stock Prices** - Current stock prices via MarketStack API
- **Historical Data** - Daily historical stock data with price changes and volume
- **Intelligent Caching** - Redis-based caching to reduce API calls:
  - Stock prices: 15-minute TTL
  - Historical data: 24-hour TTL
- **Rate Limiting** - Per-user and per-IP rate limiting via Redis sliding window

### Financial Tools

- **Portfolio Calculator** - Calculate potential gains/losses with projected prices
- **Compound Interest Calculator** - Calculate future value with monthly contributions
- **Portfolio Analytics** - View total portfolio value, cash, and net worth

### User Experience

- **Responsive Design** - Mobile-friendly UI using Bootstrap
- **Type Safety** - Full TypeScript coverage for better developer experience
- **Error Handling** - Comprehensive error handling with user-friendly messages
- **Loading States** - Visual feedback during API operations
- **Protected Routes** - Authentication-required routes with automatic redirects

---

## Project Structure

```
PaperTrader/
├── backend/
│   ├── Dockerfile                    # Backend container definition
│   ├── go.mod                        # Go module dependencies
│   ├── go.sum                        # Dependency checksums
│   ├── main.go                       # Application entry point
│   └── internal/
│       ├── api/                      # HTTP handlers and routing
│       │   ├── account/              # Account management endpoints
│       │   │   ├── handler.go        # Registration, login, profile handlers
│       │   │   ├── routes.go         # Account route definitions
│       │   │   └── dto.go            # Data transfer objects
│       │   ├── investments/          # Trading endpoints
│       │   │   ├── handler.go        # Buy/sell stock handlers
│       │   │   ├── routes.go         # Investment route definitions
│       │   │   └── dto.go            # Trade DTOs
│       │   ├── market/               # Market data endpoints
│       │   │   ├── stockHandler.go   # Stock price and historical data handlers
│       │   │   ├── routes.go         # Market route definitions
│       │   │   └── dto.go            # Market data DTOs
│       │   ├── auth/                 # Authentication middleware
│       │   │   └── middleware.go     # JWT middleware
│       │   └── middleware/           # HTTP middleware
│       │       ├── cors.go           # CORS configuration
│       │       └── rate_limit.go     # Rate limiting middleware
│       ├── config/                   # Configuration management
│       │   ├── config.go             # Config struct and loading
│       │   ├── postgres.go           # PostgreSQL connection
│       │   └── redis.go              # Redis connection
│       ├── data/                     # Data access layer
│       │   ├── user.go               # User model
│       │   ├── user_store.go         # User database operations
│       │   ├── trade.go              # Trade model
│       │   ├── trade_store.go        # Trade database operations
│       │   ├── portfolio_store.go    # Portfolio/holdings operations
│       │   ├── stock_store.go        # Stock management operations
│       │   └── dbtx.go               # Database transaction interface
│       └── service/                  # Business logic layer
│           ├── auth.go               # Authentication service
│           ├── jwt.go                # JWT token service
│           ├── market.go             # Market data service
│           ├── investment.go         # Trading service (buy/sell logic)
│           ├── stock_cache.go        # Stock price caching interface
│           ├── historical_cache.go   # Historical data caching interface
│           ├── rate_limiter.go       # Rate limiting interface
│           └── errors.go             # Service error definitions
│
├── frontend/
│   ├── Dockerfile                    # Frontend container definition
│   ├── package.json                  # Node.js dependencies
│   ├── tsconfig.json                 # TypeScript configuration
│   ├── public/                       # Static assets
│   │   └── index.html                # HTML template
│   └── src/
│       ├── components/               # React components (feature-based)
│       │   ├── auth/                 # Authentication components
│       │   │   ├── Login.tsx
│       │   │   └── Register.tsx
│       │   ├── trading/              # Trading features
│       │   │   ├── Dashboard.tsx     # Portfolio dashboard
│       │   │   ├── Trade.tsx         # Buy/sell interface
│       │   │   └── Markets.tsx       # Market listings (placeholder)
│       │   ├── tools/                # Financial calculators
│       │   │   ├── Calculator.tsx    # Portfolio calculator
│       │   │   └── CompoundInterest.tsx
│       │   ├── layout/               # Layout components
│       │   │   └── Navbar.tsx        # Navigation bar
│       │   └── common/               # Shared components
│       │       ├── Home.tsx          # Landing page
│       │       ├── Stock.tsx         # Stock detail view (placeholder)
│       │       └── ProtectedRoute.tsx # Route protection wrapper
│       ├── hooks/                    # Custom React hooks
│       │   ├── useAuth.ts            # Authentication state management
│       │   └── useApi.ts             # Generic API hook
│       ├── services/                 # API services
│       │   └── api.ts                # Type-safe API client
│       ├── types/                    # TypeScript type definitions
│       │   ├── api.ts                # API request/response types
│       │   ├── errors.ts             # Error handling types
│       │   └── index.ts              # Type exports
│       ├── utils/                    # Utility functions
│       │   └── validation.ts         # Form validation utilities
│       ├── App.tsx                   # Main app component
│       ├── index.tsx                 # Application entry point
│       ├── App.css                   # Application styles
│       ├── index.css                 # Global styles
│       └── setupProxy.ts             # Development proxy configuration
│
├── docker-compose.yml                # Production Docker Compose
├── docker-compose.dev.yml            # Development Docker Compose
├── Caddyfile                         # Caddy reverse proxy configuration
├── .gitignore                        # Git ignore rules
└── README.md                         # This file
```

---

## Setup & Installation

### Prerequisites

- **Docker** and **Docker Compose** (for containerized setup)
- **Go 1.23+** (for local backend development)
- **Node.js 18+** and **npm** (for local frontend development)
- **PostgreSQL 15+** (for local database, or use Docker)
- **Redis 7+** (for local cache, or use Docker)
- **MarketStack API Key** (get one at [marketstack.com](https://marketstack.com))


### Development Setup (Docker Compose)

The development environment includes hot-reload for both frontend and backend:

```bash
# Start all services with hot-reload
docker compose -f docker-compose.dev.yml up --build

# Or run in detached mode
docker compose -f docker-compose.dev.yml up -d --build

# View logs
docker compose -f docker-compose.dev.yml logs -f

# Stop services
docker compose -f docker-compose.dev.yml down
```

**Services**:
- **Backend**: `http://localhost:8080` (hot-reload with volume mounts)
- **Frontend**: `http://localhost:3000` (React dev server with hot-reload)
- **PostgreSQL**: `localhost:5432`
- **Redis**: `localhost:6379`

### Production Setup (Docker Compose)

The production environment uses pre-built Docker images and includes Caddy reverse proxy:

```bash
# Build images first (if building locally)
docker build -t papertrader-backend:latest ./backend
docker build -t papertrader-frontend:latest ./frontend

# Start all services
docker compose up -d --build

# View logs
docker compose logs -f

# Stop services
docker compose down
```

**Services**:
- **Caddy (Reverse Proxy)**: `http://localhost:80` and `https://localhost:443`
- **Backend**: Internal network only (via Caddy)
- **Frontend**: Internal network only (via Caddy)
- **PostgreSQL**: Internal network only
- **Redis**: Internal network only

### Local Development (Without Docker)

#### Backend Setup

```bash
cd backend

# Install dependencies
go mod download

# Run the server
go run main.go
```

#### Frontend Setup

```bash
cd frontend

# Install dependencies
npm install

# Start development server
npm start
```

The frontend will start at `http://localhost:3000` and proxy API requests to `http://localhost:8080`.

---

## API Documentation

Complete API documentation is available in [API.md](API.md).

The API provides endpoints for:
- **Authentication** - User registration, login, logout, and profile management
- **Trading** - Buy/sell stocks and portfolio management
- **Market Data** - Real-time stock prices and historical data

All endpoints use JSON for request/response bodies and JWT tokens for authentication.

---

## Database Schema

Complete database schema documentation is available in [DATABASE_SCHEMA.md](DATABASE_SCHEMA.md).

The database consists of:
- **PostgreSQL** - Primary database for users, trades, and portfolio data
- **Redis** - Caching layer for stock prices and rate limiting

Key tables:
- `users` - User accounts and authentication
- `trades` - Transaction history (event log)
- `portfolio` - Current stock holdings (materialized view)
- `stocks` - Stock metadata (optional)

---

## Configuration

### Backend Configuration

All configuration is loaded from environment variables (see [Environment Variables](#environment-variables)).

Key configuration options:
- `PORT` - Server port (default: 8080)
- `DATABASE_URL` - PostgreSQL connection string
- `JWT_SECRET` - Secret key for JWT signing (change in production!)
- `FRONTEND_URL` - Allowed CORS origin
- `MARKETSTACK_API_KEY` - MarketStack API key
- `REDIS_URL` - Redis connection URL

### Frontend Configuration

- `REACT_APP_API_URL` - API base URL (default: `/api` for proxy, or full URL)

### Caddy Configuration

The `Caddyfile` configures:
- Automatic HTTPS with Let's Encrypt
- Reverse proxy routing:
  - `/api/*` → Backend (port 8080)
  - `/*` → Frontend (port 80)
- Email for SSL certificate notifications

---

## Development

### Backend Development

**Adding a New Endpoint**:

1. Create handler in `internal/api/{feature}/handler.go`
2. Define routes in `internal/api/{feature}/routes.go`
3. Add route registration in `main.go`
4. Implement business logic in `internal/service/`
5. Add data access methods in `internal/data/`

**Running Tests**:
```bash
cd backend
go test ./...
```

**Code Style**:
- Use `gofmt` for formatting
- Follow Go naming conventions
- Add comments for exported functions/types

### Frontend Development

**Adding a New Component**:

1. Create component in appropriate feature folder (`components/{feature}/`)
2. Define TypeScript interfaces for props
3. Add route in `App.tsx` if needed
4. Update types in `types/` if new API types are needed

**Type Checking**:
```bash
cd frontend
npm run type-check
```

**Code Style**:
- Use TypeScript strict mode
- Define interfaces for all props and state
- Use functional components with hooks
- Follow the established folder structure


---


### Docker Production Build

```bash
# Build backend image
cd backend
docker build -t papertrader-backend:latest .

# Build frontend image
cd ../frontend
docker build -t papertrader-frontend:latest .

# Or use docker-compose build
docker compose build
```

### Environment-Specific Configurations

**Development** (`docker-compose.dev.yml`):
- Volume mounts for hot-reload
- Exposed ports for direct access
- Development-friendly timeouts
- Polling enabled for file watching

**Production** (`docker-compose.yml`):
- Pre-built images
- Internal networking only
- Caddy reverse proxy
- Health checks
- Resource limits
- Persistent volumes

---

## Security

### Authentication Security

- **Password Hashing**: bcrypt with cost factor 10
- **JWT Tokens**: Signed with HS256 algorithm
- **Token Expiration**: Configured in JWT service
- **Secure Cookies**: HTTP-only cookies for token storage (optional)

### API Security

- **CORS**: Configured to allow only frontend origin
- **Rate Limiting**: Per-user and per-IP limits via Redis
- **Input Validation**: All inputs validated in handlers
- **SQL Injection Prevention**: Parameterized queries only
- **Error Handling**: Generic error messages (no stack traces in production)

### Data Security

- **ACID Transactions**: All financial operations are transactional
- **Precision**: Financial values rounded to 2 decimal places
- **Balance Validation**: Checks before allowing trades
- **Portfolio Integrity**: Unique constraints prevent duplicate holdings

### Infrastructure Security

- **HTTPS**: Automatic SSL/TLS via Caddy
- **Network Isolation**: Services on internal Docker network
- **Secret Management**: Environment variables (consider secret manager for production)
- **Database**: No direct external access (internal network only)

---


### Logs

**View all logs**:
```bash
docker compose logs -f
```

**View specific service**:
```bash
docker compose logs -f backend
docker compose logs -f frontend
docker compose logs -f postgres
docker compose logs -f redis
```


---

## License

This project is open source and available under the MIT License.
