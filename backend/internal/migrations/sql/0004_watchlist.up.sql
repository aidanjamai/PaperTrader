CREATE TABLE IF NOT EXISTS watchlist (
	id VARCHAR(255) PRIMARY KEY,
	user_id VARCHAR(255) NOT NULL REFERENCES users(id),
	symbol VARCHAR(10) NOT NULL,
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(user_id, symbol)
);

CREATE INDEX IF NOT EXISTS idx_watchlist_user_id ON watchlist(user_id);
