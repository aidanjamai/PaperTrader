-- Persisted daily EOD closes per symbol. Lets us serve stock-detail charts
-- from Postgres on subsequent loads instead of burning MarketStack quota.
CREATE TABLE IF NOT EXISTS stock_history (
    symbol VARCHAR(10) NOT NULL,
    trade_date DATE NOT NULL,
    close NUMERIC(20, 8) NOT NULL,
    volume BIGINT NOT NULL DEFAULT 0,
    fetched_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (symbol, trade_date)
);

-- Range scans for the chart query (`WHERE symbol = $1 AND trade_date BETWEEN ...`)
-- are already covered by the primary key, so no additional index needed.
