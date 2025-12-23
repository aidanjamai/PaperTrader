# Database Schema Documentation

This document describes the database schema for the PaperTrader application, including PostgreSQL tables and Redis key patterns.

## Table of Contents

- [PostgreSQL Schema](#postgresql-schema)
- [Redis Keys](#redis-keys)
- [Indexes](#indexes)
- [Relationships](#relationships)

---

## PostgreSQL Schema

### `users`

Stores user account information including authentication credentials and account balance.

```sql
CREATE TABLE users (
    id VARCHAR(255) PRIMARY KEY,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    balance NUMERIC(15,2) NOT NULL DEFAULT 10000.00,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

**Columns**:
- `id` - UUID string, primary key
- `email` - Unique email address for authentication
- `password_hash` - bcrypt hashed password
- `balance` - Account balance for paper trading (default: $10,000.00)
- `created_at` - Timestamp of account creation

**Indexes**:
- Primary key on `id`
- Unique constraint on `email`

---

### `trades`

Records all buy and sell transactions executed by users. Acts as an event log for trading history.

```sql
CREATE TABLE trades (
    id VARCHAR(255) PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL REFERENCES users(id),
    symbol VARCHAR(10) NOT NULL,
    action VARCHAR(10) NOT NULL, -- 'BUY' or 'SELL'
    quantity INTEGER NOT NULL,
    price NUMERIC(15,2) NOT NULL,
    date VARCHAR(50) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'COMPLETED',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

**Columns**:
- `id` - UUID string, primary key
- `user_id` - Foreign key to `users.id`
- `symbol` - Stock symbol (e.g., 'AAPL', 'GOOGL')
- `action` - Trade action: 'BUY' or 'SELL'
- `quantity` - Number of shares traded
- `price` - Price per share at time of trade
- `date` - Date of the trade (formatted string)
- `status` - Trade status: 'PENDING', 'COMPLETED', 'FAILED' (default: 'COMPLETED')
- `created_at` - Timestamp of trade record creation

**Indexes**:
- Primary key on `id`
- Foreign key constraint on `user_id` referencing `users(id)`

**Notes**:
- All trades are recorded in an immutable event log format
- Status field allows for transaction tracking and potential reconciliation
- Used for trade history and audit trails

---

### `portfolio`

Materialized view of user stock holdings. Aggregates buy/sell trades to show current positions.

```sql
CREATE TABLE portfolio (
    id VARCHAR(255) PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL REFERENCES users(id),
    symbol VARCHAR(10) NOT NULL,
    quantity INTEGER NOT NULL DEFAULT 0,
    avg_price NUMERIC(15,2) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, symbol)
);
```

**Columns**:
- `id` - UUID string, primary key
- `user_id` - Foreign key to `users.id`
- `symbol` - Stock symbol (e.g., 'AAPL', 'GOOGL')
- `quantity` - Total number of shares held
- `avg_price` - Average purchase price per share (weighted average)
- `created_at` - Timestamp of first position creation
- `updated_at` - Timestamp of last position update
- `UNIQUE(user_id, symbol)` - Ensures one portfolio entry per user per stock

**Indexes**:
- Primary key on `id`
- Foreign key constraint on `user_id` referencing `users(id)`
- Unique constraint on (`user_id`, `symbol`)

**Notes**:
- Maintained via upsert operations on buy/sell
- Average price is calculated as weighted average when buying
- Quantity decreases when selling
- Row is removed when quantity reaches zero
- Used for fast portfolio queries and dashboard displays

---

### `stocks` (Optional)

Stores metadata about stocks. Currently optional and can be used for stock information lookup.

```sql
CREATE TABLE stocks (
    symbol VARCHAR(10) PRIMARY KEY,
    name VARCHAR(255),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

**Columns**:
- `symbol` - Stock symbol, primary key
- `name` - Full company name
- `created_at` - Timestamp of record creation

**Indexes**:
- Primary key on `symbol`

**Notes**:
- This table is optional and may not be actively used
- Can be populated with stock metadata for reference
- Useful for displaying full company names in UI

---

## Redis Keys

Redis is used for caching and rate limiting. All keys follow a consistent naming pattern.

### Stock Price Cache

**Pattern**: `stock:{symbol}:{date}`

**Example**: `stock:AAPL:2024-01-15`

**TTL**: 15 minutes

**Value**: JSON string containing stock price data

**Purpose**: Reduces API calls to MarketStack by caching current stock prices for 15 minutes.

---

### Historical Data Cache

**Pattern**: `historical:{symbol}:{date}`

**Example**: `historical:AAPL:2024-01-15`

**TTL**: 24 hours

**Value**: JSON string containing historical stock data (price, volume, change, etc.)

**Purpose**: Caches historical stock data for a full day since it doesn't change once the trading day ends.

---

### Rate Limiting Keys

**Per-User Pattern**: `ratelimit:user:{user_id}`

**Example**: `ratelimit:user:550e8400-e29b-41d4-a716-446655440000`

**Per-IP Pattern**: `ratelimit:ip:{ip_address}`

**Example**: `ratelimit:ip:192.168.1.100`

**TTL**: Sliding window (typically 1 minute)

**Value**: Counter (integer) representing number of requests in the window

**Purpose**: Implements sliding window rate limiting to prevent API abuse. Separate limits for authenticated users and IP addresses.

**Implementation**: Uses Redis INCR with expiration to implement sliding window algorithm.

---

## Indexes

### Existing Indexes

1. **Primary Keys**: All tables have primary key indexes on their `id` or `symbol` columns
2. **Unique Constraints**: 
   - `users.email` - Ensures unique email addresses
   - `portfolio(user_id, symbol)` - Ensures one portfolio entry per user per stock
3. **Foreign Keys**: Indexed automatically by PostgreSQL
   - `trades.user_id` → `users.id`
   - `portfolio.user_id` → `users.id`

### Recommended Additional Indexes

For production environments, consider adding:

```sql
-- Index for querying trades by user
CREATE INDEX idx_trades_user_id ON trades(user_id);

-- Index for querying trades by symbol
CREATE INDEX idx_trades_symbol ON trades(symbol);

-- Index for querying portfolio by user
CREATE INDEX idx_portfolio_user_id ON portfolio(user_id);

-- Index for querying trades by date range
CREATE INDEX idx_trades_created_at ON trades(created_at);
```

---

## Relationships

### Entity Relationship Diagram

```
users (1) ──< (many) trades
  │
  └──< (many) portfolio
```

**Relationships**:
- One user can have many trades (`users.id` → `trades.user_id`)
- One user can have many portfolio entries (`users.id` → `portfolio.user_id`)
- Each portfolio entry represents one stock holding per user
- Trades are immutable event records
- Portfolio is a materialized/aggregated view of trades

---

## Data Flow

### Buy Stock Flow

1. User initiates buy request
2. System checks user balance (`users.balance`)
3. System fetches current stock price (from MarketStack API or Redis cache)
4. System calculates total cost
5. PostgreSQL transaction begins:
   - Deduct balance from `users.balance`
   - Create trade record in `trades` table (action='BUY')
   - Update or insert portfolio entry in `portfolio` table (upsert)
6. Transaction commits (all or nothing)
7. Stock price cached in Redis for 15 minutes

### Sell Stock Flow

1. User initiates sell request
2. System checks portfolio for sufficient quantity (`portfolio.quantity`)
3. System fetches current stock price (from MarketStack API or Redis cache)
4. System calculates total value
5. PostgreSQL transaction begins:
   - Add balance to `users.balance`
   - Create trade record in `trades` table (action='SELL')
   - Update portfolio entry in `portfolio` table (decrease quantity)
   - Delete portfolio entry if quantity reaches zero
6. Transaction commits (all or nothing)
7. Stock price cached in Redis for 15 minutes

---

## Data Types

### Numeric Precision

All monetary values use `NUMERIC(15,2)`:
- **Precision**: 15 total digits
- **Scale**: 2 decimal places
- **Range**: -999,999,999,999,999.99 to 999,999,999,999,999.99
- **Purpose**: Ensures exact decimal representation (no floating-point errors)

### Timestamps

All timestamp columns use `TIMESTAMP`:
- Stored in UTC
- Format: `YYYY-MM-DD HH:MM:SS`
- Default: `CURRENT_TIMESTAMP` for creation times

### String Lengths

- **UUID**: `VARCHAR(255)` (UUIDs are 36 characters, but allows for flexibility)
- **Email**: `VARCHAR(255)` (sufficient for most email addresses)
- **Symbol**: `VARCHAR(10)` (stock symbols are typically 1-5 characters)
- **Action**: `VARCHAR(10)` ('BUY' or 'SELL')
- **Status**: `VARCHAR(20)` (allows for future status types)

---

## Migration Notes

### Current Implementation

Tables are currently auto-created on application startup. The initialization code is in:
- `backend/internal/data/user_store.go` - Creates `users` table
- `backend/internal/data/trade_store.go` - Creates `trades` table
- `backend/internal/data/portfolio_store.go` - Creates `portfolio` table
- `backend/internal/data/stock_store.go` - Creates `stocks` table (optional)

### Production Recommendations

For production environments:

1. **Use Migration Tools**: Consider using migration tools like:
   - `golang-migrate`
   - `sql-migrate`
   - Custom migration scripts

2. **Version Control**: Store migration files in version control

3. **Backup Strategy**: Implement regular database backups before migrations

4. **Testing**: Test migrations on staging environment first

5. **Rollback Plan**: Ensure migrations are reversible or have rollback procedures

---

## Performance Considerations

### Query Optimization

- **Portfolio Queries**: Indexed on `user_id` for fast user portfolio retrieval
- **Trade History**: Consider pagination for users with many trades
- **Stock Price Cache**: Redis cache reduces database load significantly

### Scaling Considerations

- **Read Replicas**: For high read traffic, consider PostgreSQL read replicas
- **Connection Pooling**: Backend uses connection pooling (configured in `config/postgres.go`)
- **Redis Clustering**: For high availability, consider Redis Cluster
- **Partitioning**: For very large datasets, consider table partitioning by date or user_id

---

## Security Considerations

### Data Protection

- **Passwords**: Never stored in plain text, always bcrypt hashed
- **Financial Data**: All monetary values are precise (NUMERIC type)
- **Transactions**: ACID guarantees ensure data integrity

### Access Control

- **Row-Level Security**: All queries filtered by `user_id` from JWT token
- **Foreign Key Constraints**: Prevent orphaned records
- **Unique Constraints**: Prevent duplicate data (email, portfolio entries)

---

## Backup and Recovery

### Backup Strategy

1. **PostgreSQL**: Use `pg_dump` for regular backups
2. **Redis**: Consider persistence options (RDB snapshots, AOF)
3. **Frequency**: Daily backups recommended for production

### Recovery Procedures

1. Restore PostgreSQL from latest backup
2. Redis cache will rebuild automatically on first requests
3. Verify data integrity after recovery

---

## Future Enhancements

Potential schema additions:

- **Watchlists**: Tables for user-created stock watchlists
- **Alerts**: Price alert thresholds and notifications
- **Orders**: Pending orders (limit orders, stop orders)
- **Analytics**: Aggregated performance metrics per user
- **Social Features**: Sharing trades, following other users
- **Audit Log**: Separate audit table for all financial transactions
