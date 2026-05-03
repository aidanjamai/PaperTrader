CREATE TABLE IF NOT EXISTS trades (
	id VARCHAR(255) PRIMARY KEY,
	user_id VARCHAR(255) NOT NULL,
	symbol VARCHAR(10) NOT NULL,
	action VARCHAR(10) NOT NULL,
	quantity INTEGER NOT NULL,
	price NUMERIC(15,2) NOT NULL,
	date VARCHAR(20),
	status VARCHAR(20) NOT NULL DEFAULT 'COMPLETED'
);

ALTER TABLE trades ADD COLUMN IF NOT EXISTS executed_at TIMESTAMPTZ;

DO $$
BEGIN
	IF EXISTS (
		SELECT 1 FROM information_schema.columns
		WHERE table_name = 'trades' AND column_name = 'date'
	) THEN
		EXECUTE 'UPDATE trades
			SET executed_at = TO_TIMESTAMP(date, ''MM/DD/YYYY'')
			WHERE executed_at IS NULL AND date IS NOT NULL';
	END IF;
END $$;

ALTER TABLE trades ALTER COLUMN executed_at SET DEFAULT CURRENT_TIMESTAMP;

UPDATE trades SET executed_at = CURRENT_TIMESTAMP WHERE executed_at IS NULL;

ALTER TABLE trades ALTER COLUMN executed_at SET NOT NULL;

ALTER TABLE trades DROP COLUMN IF EXISTS date;

CREATE INDEX IF NOT EXISTS idx_trades_user_id_executed_at ON trades(user_id, executed_at DESC);

CREATE OR REPLACE FUNCTION reject_trade_mutation() RETURNS trigger AS $$
BEGIN
  RAISE EXCEPTION 'trades is append-only — % is not permitted', TG_OP;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trades_no_update ON trades;
CREATE TRIGGER trades_no_update
  BEFORE UPDATE ON trades
  FOR EACH ROW EXECUTE FUNCTION reject_trade_mutation();

DROP TRIGGER IF EXISTS trades_no_delete ON trades;
CREATE TRIGGER trades_no_delete
  BEFORE DELETE ON trades
  FOR EACH ROW EXECUTE FUNCTION reject_trade_mutation();

ALTER TABLE trades ADD COLUMN IF NOT EXISTS idempotency_key VARCHAR(255);

CREATE UNIQUE INDEX IF NOT EXISTS idx_trades_user_idempotency_key
  ON trades(user_id, idempotency_key)
  WHERE idempotency_key IS NOT NULL;
