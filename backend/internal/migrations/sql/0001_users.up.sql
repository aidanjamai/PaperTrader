CREATE TABLE IF NOT EXISTS users (
	id VARCHAR(255) PRIMARY KEY,
	email VARCHAR(255) UNIQUE NOT NULL,
	password TEXT,
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	balance NUMERIC(15,2) DEFAULT 10000.00,
	email_verified BOOLEAN DEFAULT FALSE,
	verification_token VARCHAR(255),
	verification_token_expires TIMESTAMP,
	google_id VARCHAR(255) UNIQUE,
	created_via VARCHAR(50) DEFAULT 'email'
);

ALTER TABLE users ADD COLUMN IF NOT EXISTS email_verified BOOLEAN DEFAULT FALSE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS verification_token VARCHAR(255);
ALTER TABLE users ADD COLUMN IF NOT EXISTS verification_token_expires TIMESTAMP;
ALTER TABLE users ADD COLUMN IF NOT EXISTS google_id VARCHAR(255) UNIQUE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS created_via VARCHAR(50) DEFAULT 'email';
ALTER TABLE users ALTER COLUMN password DROP NOT NULL;

DO $$
BEGIN
	IF NOT EXISTS (
		SELECT 1 FROM pg_constraint WHERE conname = 'users_balance_non_negative'
	) THEN
		ALTER TABLE users ADD CONSTRAINT users_balance_non_negative CHECK (balance >= 0);
	END IF;
END $$;
