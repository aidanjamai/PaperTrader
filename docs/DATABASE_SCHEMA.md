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
    password TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    balance NUMERIC(15,2) DEFAULT 10000.00,
    email_verified BOOLEAN DEFAULT FALSE,
    verification_token VARCHAR(255),
    verification_token_expires TIMESTAMP,
    google_id VARCHAR(255) UNIQUE,
    created_via VARCHAR(50) DEFAULT 'email',
    CONSTRAINT users_balance_non_negative CHECK (balance >= 0)
);
```

**Columns**:
- `id` - UUID string, primary key
- `email` - Unique email address for authentication
- `password` - bcrypt hashed password. **Nullable** — Google OAuth users have no local password
- `created_at` - Timestamp of account creation
- `balance` - Account balance for paper trading (default: $10,000.00). Constrained to be non-negative
- `email_verified` - Whether the user has confirmed their email address (default: `FALSE`)
- `verification_token` - One-time token emailed to the user for email verification
- `verification_token_expires` - Expiry timestamp for the verification token
- `google_id` - Google OAuth subject identifier (unique). `NULL` for email/password users
- `created_via` - Account origin marker, e.g. `'email'` or `'google'` (default: `'email'`)

**Indexes / Constraints**:
- Primary key on `id`
- Unique constraint on `email`
- Unique constraint on `google_id`
- `CHECK (balance >= 0)` via `users_balance_non_negative`

---

### `trades`

Records all buy and sell transactions executed by users. Acts as an event log for trading history.

```sql
CREATE TABLE trades (
    id VARCHAR(255) PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL,
    symbol VARCHAR(10) NOT NULL,
    action VARCHAR(10) NOT NULL, -- 'BUY' or 'SELL'
    quantity INTEGER NOT NULL,
    price NUMERIC(15,2) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'COMPLETED',
    executed_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    idempotency_key VARCHAR(255)
);
```

**Columns**:
- `id` - UUID string, primary key
- `user_id` - Owning user's id (no FK declared at the table level — see Relationships)
- `symbol` - Stock symbol (e.g., 'AAPL', 'GOOGL')
- `action` - Trade action: 'BUY' or 'SELL'
- `quantity` - Number of shares traded
- `price` - Price per share at time of trade
- `status` - Trade status: 'PENDING', 'COMPLETED', 'FAILED' (default: 'COMPLETED')
- `executed_at` - Timestamp (with time zone) of when the trade was executed; defaults to `CURRENT_TIMESTAMP` and is `NOT NULL`
- `idempotency_key` - Optional client-supplied key used to deduplicate retried buy/sell requests. Nullable

**Indexes**:
- Primary key on `id`
- `idx_trades_user_id_executed_at` on `(user_id, executed_at DESC)` — primary index for trade-history queries
- `idx_trades_user_idempotency_key` — UNIQUE partial index on `(user_id, idempotency_key) WHERE idempotency_key IS NOT NULL`

**Triggers (append-only enforcement)**:

The `trades` table is **append-only at the database level**. Two `BEFORE` triggers
call `reject_trade_mutation()`, which raises an exception:

- `trades_no_update` — `BEFORE UPDATE ON trades`
- `trades_no_delete` — `BEFORE DELETE ON trades`

Any `UPDATE` or `DELETE` against `trades` will fail with
`trades is append-only — UPDATE/DELETE is not permitted`.

**Notes**:
- All trades are recorded in an immutable event log format (DB-enforced via triggers above)
- Status field allows for transaction tracking and potential reconciliation
- Used for trade history and audit trails
- The legacy `date VARCHAR(50)` column was removed in migration `0002_trades.up.sql`; data was backfilled into `executed_at` before the column was dropped

---

### `portfolio`

Materialized view of user stock holdings. Aggregates buy/sell trades to show current positions.

```sql
CREATE TABLE portfolio (
    id VARCHAR(255) PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL,
    symbol VARCHAR(10) NOT NULL,
    quantity INTEGER NOT NULL DEFAULT 0,
    avg_price NUMERIC(20,8) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, symbol)
);
```

**Columns**:
- `id` - UUID string, primary key
- `user_id` - Owning user's id (no FK declared at the table level — see Relationships)
- `symbol` - Stock symbol (e.g., 'AAPL', 'GOOGL')
- `quantity` - Total number of shares held
- `avg_price` - Average purchase price per share (weighted average). `NUMERIC(20,8)` — widened from `NUMERIC(15,2)` in migration `0006_widen_portfolio_avg_price` to preserve precision for low-priced or fractional cost bases
- `created_at` - Timestamp of first position creation
- `updated_at` - Timestamp of last position update
- `UNIQUE(user_id, symbol)` - Ensures one portfolio entry per user per stock

**Indexes**:
- Primary key on `id`
- `idx_portfolio_user_id` on `user_id`
- Unique constraint on (`user_id`, `symbol`)

**Notes**:
- Maintained via upsert operations on buy/sell
- Average price is calculated as weighted average when buying
- Quantity decreases when selling
- Row is removed when quantity reaches zero
- Used for fast portfolio queries and dashboard displays

---

### `watchlist`

Stores symbols a user is tracking without holding shares. Used by the dashboard
Watchlist card and the Markets trade modal toggle.

```sql
CREATE TABLE watchlist (
    id VARCHAR(255) PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL REFERENCES users(id),
    symbol VARCHAR(10) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, symbol)
);
```

**Columns**:
- `id` - UUID string, primary key
- `user_id` - Foreign key to `users.id`
- `symbol` - Stock symbol (e.g., 'AAPL', 'GOOGL')
- `created_at` - Timestamp the symbol was added to the watchlist
- `UNIQUE(user_id, symbol)` - One row per user per symbol

**Indexes**:
- Primary key on `id`
- Foreign key constraint on `user_id` referencing `users(id)`
- `idx_watchlist_user_id` on `user_id` for fast per-user lookups
- Unique constraint on (`user_id`, `symbol`)

**Notes**:
- `POST /api/watchlist` validates the symbol against MarketStack before insert
  (rejects unknown symbols with 404). Duplicates return 409.
- Listing enriches each row with current price/change via the same batch
  historical endpoint used by the portfolio dashboard (24-hour Redis cache).

---

### `stock_history`

Persisted daily EOD closes per symbol. Backs the stock-detail price chart so
repeat loads of the same symbol serve from Postgres rather than burning
MarketStack quota. Independent of users — the same row is shared by every
user that views a given symbol's chart.

```sql
CREATE TABLE stock_history (
    symbol VARCHAR(10) NOT NULL,
    trade_date DATE NOT NULL,
    close NUMERIC(20, 8) NOT NULL,
    volume BIGINT NOT NULL DEFAULT 0,
    fetched_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (symbol, trade_date)
);
```

**Columns**:
- `symbol` - Stock symbol (uppercased; matches the `util.ValidateSymbol` shape)
- `trade_date` - The trading day this close belongs to
- `close` - End-of-day close price; `NUMERIC(20,8)` matches `portfolio.avg_price` so weighted-average math stays exact
- `volume` - Reported daily share volume; `BIGINT` because high-volume tickers exceed `INTEGER`
- `fetched_at` - Most recent time the row was upserted (refreshed on conflict)

**Indexes**:
- `PRIMARY KEY (symbol, trade_date)` — covers all chart range scans (`WHERE symbol = $1 AND trade_date BETWEEN ...`); no secondary index needed.

**Notes**:
- Written by `MarketService.GetHistoricalSeries` (`backend/internal/service/market.go`) via batched upserts in `data.StockHistoryStore.UpsertMany`, which chunks at `upsertBatchSize = 1000` to stay well under Postgres's 65,535-parameter ceiling.
- On conflict on `(symbol, trade_date)`, `close` / `volume` / `fetched_at` are refreshed — the latest value always wins.
- No FK to `users` — this is a shared cache table, not user-owned data. There is no per-user view; every user reading AAPL's chart hits the same rows.

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

**Pattern**: `historical:{symbol}:{startDate}:{endDate}`

**Example**: `historical:AAPL:2024-01-01:2024-01-15`

**TTL**: 24 hours

**Value**: JSON string containing historical stock data (price, volume, change, etc.) for the requested range

**Purpose**: Caches historical stock data for a full day since it doesn't change once the trading day ends. Keying by both endpoints of the date range lets distinct queries (e.g. 7-day vs 30-day) coexist without colliding.

---

### Empty-Range Negative Cache

**Pattern**: `historical-empty:{symbol}:{from}:{to}`

**Example**: `historical-empty:AAPL:2026-05-02:2026-05-03`

**TTL**: 6 hours

**Value**: Sentinel string `"1"`

**Purpose**: Memoizes `(symbol, range)` combinations that MarketStack returned zero rows for. Without this, every chart load on a Saturday or Sunday would fire a fresh MarketStack call for the same Sat→Sun gap and get the same empty response back, burning quota indefinitely. The 6-hour TTL is short enough that a Tuesday-morning request still picks up Monday's close as soon as MarketStack publishes it.

**Set by**: `MarketService.fillGap` via `RedisHistoricalCache.MarkRangeEmpty` (`backend/internal/service/market.go`).

**Read by**: `MarketService.fillGap` via `RedisHistoricalCache.IsRangeEmpty` — when the marker is present, the gap fetch is skipped.

---

### Rate Limiting Keys

**Per-User Pattern**: `ratelimit:user:{user_id}`

**Example**: `ratelimit:user:550e8400-e29b-41d4-a716-446655440000`

**Per-IP Pattern**: `ratelimit:ip:{ip_address}`

**Example**: `ratelimit:ip:192.168.1.100`

**TTL**: Sliding window (default 1 hour; key TTL is `window + 1 minute`)

**Value**: Redis **sorted set**. Each member is a request, scored by its arrival time in nanoseconds (`time.Now().UnixNano()`).

**Purpose**: Implements sliding window rate limiting to prevent API abuse. Separate limits for authenticated users and IP addresses (defaults: 100 req/hour per user, 200 req/hour per IP).

**Implementation**: For every request the limiter pipelines four commands against the relevant key:

1. `ZREMRANGEBYSCORE key 0 <windowStart>` — evict entries older than the window
2. `ZCARD key` — count remaining entries (the in-window request count)
3. `ZADD key <nowNs> <nowNs>` — record the current request
4. `EXPIRE key <window+1m>` — keep the key from leaking if traffic stops

If `ZCARD` already meets or exceeds the limit, the request is rejected. Redis errors fail open (request allowed). See `backend/internal/service/rate_limiter.go`.

---

## Indexes

### Existing Indexes

1. **Primary Keys**: All tables have primary key indexes on their `id` column
2. **Unique Constraints**:
   - `users.email`
   - `users.google_id`
   - `portfolio(user_id, symbol)`
   - `watchlist(user_id, symbol)`
   - `trades(user_id, idempotency_key) WHERE idempotency_key IS NOT NULL` (`idx_trades_user_idempotency_key`)
   - `stock_history(symbol, trade_date)` — composite primary key
3. **CHECK Constraints**:
   - `users_balance_non_negative` — `CHECK (balance >= 0)`
4. **Secondary Indexes**:
   - `idx_trades_user_id_executed_at` on `trades(user_id, executed_at DESC)`
   - `idx_portfolio_user_id` on `portfolio(user_id)`
   - `idx_watchlist_user_id` on `watchlist(user_id)`
5. **Foreign Keys**: Only `watchlist.user_id` declares `REFERENCES users(id)` (see Relationships). `stock_history` has no FK — it is a shared cache table independent of users.

### Foreign Keys

Despite the conceptual relationship, `trades.user_id` and `portfolio.user_id` are
**not** declared as `REFERENCES users(id)` in their CREATE TABLE statements
(`0002_trades.up.sql`, `0003_portfolio.up.sql`). Only `watchlist.user_id` has
the FK constraint (`0004_watchlist.up.sql`). Application-level integrity is
relied upon for trades and portfolio.

### Recommended Additional Indexes

For production environments, consider adding:

```sql
-- Index for querying trades by symbol (cross-user analytics)
CREATE INDEX idx_trades_symbol ON trades(symbol);
```

The previously suggested indexes on `trades(user_id)`, `portfolio(user_id)`, and
`trades(created_at)` are either already in place via
`idx_trades_user_id_executed_at` / `idx_portfolio_user_id`, or no longer
applicable (`trades.created_at` does not exist — use `executed_at`).

---

## Relationships

### Entity Relationship Diagram

```
users (1) ──< (many) trades
  │
  ├──< (many) portfolio
  │
  └──< (many) watchlist

stock_history    (shared cache, no FK to users)
```

**Relationships**:
- One user can have many trades (`users.id` → `trades.user_id`, **enforced in application code only — no DB-level FK**)
- One user can have many portfolio entries (`users.id` → `portfolio.user_id`, **enforced in application code only — no DB-level FK**)
- One user can have many watchlist entries (`users.id` → `watchlist.user_id`, **DB-level `REFERENCES users(id)`**)
- Each portfolio entry represents one stock holding per user
- Each watchlist entry represents one tracked symbol per user
- Trades are immutable event records (DB-enforced via the `trades_no_update` and `trades_no_delete` triggers)
- Portfolio is a materialized/aggregated view of trades
- `stock_history` is **not** related to users — it is a shared symbol/date keyed cache populated by the chart endpoint and read by every user that views the same symbol

---

## Data Flow

### Buy Stock Flow

1. User initiates buy request, optionally supplying an `Idempotency-Key` header.
2. **Idempotency short-circuit**: if a trade with the same `(user_id, idempotency_key)` already exists, the prior trade's response is returned without re-executing.
3. System fetches the current stock price (from MarketStack API or Redis cache) and computes total cost.
4. PostgreSQL transaction begins:
   - `SELECT balance FROM users WHERE id = $1 FOR UPDATE` — row-locks the user's balance so two concurrent buys cannot both pass the funds check (`service/investment.go`).
   - If `balance < totalPrice`, the transaction rolls back and the API returns **HTTP 402 Payment Required** (`insufficient funds`).
   - Deduct balance from `users.balance`.
   - Insert trade record into `trades` (action=`'BUY'`, `executed_at` defaulted by the DB, `idempotency_key` persisted if supplied).
   - Upsert portfolio entry in `portfolio` (weighted-average price recomputed).
5. Transaction commits (all or nothing).
6. **Append-only enforcement**: trades cannot be updated or deleted; the `trades_no_update` / `trades_no_delete` triggers raise an exception on any such attempt.
7. If two concurrent retries with the same idempotency key race, the loser hits the unique partial index, the transaction is rolled back, and the original trade is fetched and returned as the replay.

### Sell Stock Flow

1. User initiates sell request, optionally supplying an `Idempotency-Key` header.
2. **Idempotency short-circuit**: a prior trade with the same `(user_id, idempotency_key)` is replayed without re-executing.
3. System fetches the current stock price (from MarketStack API or Redis cache) and computes total proceeds.
4. PostgreSQL transaction begins:
   - `SELECT ... FOR UPDATE` row-locks the user's balance row.
   - Verify the portfolio entry has sufficient `quantity`; if not, the transaction rolls back with an error.
   - Add proceeds to `users.balance`.
   - Insert trade record into `trades` (action=`'SELL'`, `executed_at` defaulted by the DB, `idempotency_key` persisted if supplied).
   - Update portfolio entry (decrease `quantity`); delete the row when `quantity` reaches zero.
5. Transaction commits (all or nothing).
6. Trades remain immutable post-commit (DB-enforced via the append-only triggers).

---

## Data Types

### Numeric Precision

Most monetary values use `NUMERIC(15,2)`:
- **Precision**: 15 total digits
- **Scale**: 2 decimal places
- **Range**: -999,999,999,999,999.99 to 999,999,999,999,999.99
- **Purpose**: Ensures exact decimal representation (no floating-point errors)
- **Used by**: `users.balance`, `trades.price`

`portfolio.avg_price` is the exception: it is `NUMERIC(20,8)` (widened from
`NUMERIC(15,2)` in migration `0006_widen_portfolio_avg_price`). The eight
fractional digits preserve the weighted-average cost basis when low-priced or
fractional-share trades would otherwise round.

### Timestamps

Most timestamp columns use `TIMESTAMP` (without time zone) and are stored in UTC.
The exception is `trades.executed_at`, which is `TIMESTAMPTZ` so the wall-clock
of execution is preserved unambiguously across regions.

- Default: `CURRENT_TIMESTAMP` for creation/execution times

### String Lengths

- **UUID**: `VARCHAR(255)` (UUIDs are 36 characters, but allows for flexibility)
- **Email**: `VARCHAR(255)` (sufficient for most email addresses)
- **Symbol**: `VARCHAR(10)` (stock symbols are typically 1-5 characters)
- **Action**: `VARCHAR(10)` ('BUY' or 'SELL')
- **Status**: `VARCHAR(20)` (allows for future status types)

---

## Migration Notes

### Current Implementation

Schema is managed with **`golang-migrate`**. Migration SQL lives in
`backend/internal/migrations/sql/` as paired `NNNN_<name>.up.sql` /
`NNNN_<name>.down.sql` files and is the source of truth for the schema.

When `MIGRATE_ON_START=true`, the backend runs `migrations.Run(db)` during
startup (`backend/main.go` ~L259-264). Otherwise, migrations are expected to be
applied out-of-band via `cmd/migrate`.

Applied migrations to date:

| # | Name | Purpose |
|---|------|---------|
| 0001 | `users` | Create `users`; add OAuth/verification columns; relax `password NOT NULL`; add `users_balance_non_negative` CHECK |
| 0002 | `trades` | Create `trades`; backfill and switch from `date` to `executed_at TIMESTAMPTZ`; install append-only triggers; add `idempotency_key` + unique partial index |
| 0003 | `portfolio` | Create `portfolio` with `(user_id, symbol)` unique and `idx_portfolio_user_id` |
| 0004 | `watchlist` | Create `watchlist` with FK to `users(id)` and `idx_watchlist_user_id` |
| 0005 | `stocks` | Created a `stocks` table (subsequently dropped — see 0007) |
| 0006 | `widen_portfolio_avg_price` | `ALTER ... TYPE NUMERIC(20,8)` for `portfolio.avg_price` |
| 0007 | `drop_stocks` | `DROP TABLE stocks CASCADE` — the table is no longer used |
| 0008 | `stock_history` | Create `stock_history(symbol, trade_date, close, volume, fetched_at)` with `PRIMARY KEY (symbol, trade_date)` — backs the persisted chart cache |

### Production Recommendations

1. **Version Control**: All migration files are committed under
   `backend/internal/migrations/sql/`.
2. **Backup Strategy**: Take a backup before applying migrations in production.
3. **Testing**: Apply migrations on staging first.
4. **Rollback Plan**: Each `*.up.sql` has a paired `*.down.sql`; review the
   `.down.sql` before relying on it (some operations like trigger drops and
   data backfills are not perfectly reversible).

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
- **Foreign Key Constraints**: `watchlist.user_id` references `users(id)`; `trades` and `portfolio` rely on application-level integrity (no DB-level FK)
- **Unique Constraints**: Prevent duplicate data (`users.email`, `users.google_id`, `(portfolio.user_id, symbol)`, `(watchlist.user_id, symbol)`, and `(trades.user_id, idempotency_key)` for idempotent retries)
- **Append-only Trades**: `BEFORE UPDATE` and `BEFORE DELETE` triggers reject any mutation of `trades`, preserving the audit trail at the DB level

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

- **Alerts**: Price alert thresholds and notifications
- **Orders**: Pending orders (limit orders, stop orders)
- **Analytics**: Aggregated performance metrics per user
- **Social Features**: Sharing trades, following other users
- **Audit Log**: Separate audit table for all financial transactions
