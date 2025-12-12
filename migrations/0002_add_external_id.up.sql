ALTER TABLE transactions ADD COLUMN external_id TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS idx_transactions_external_id ON transactions(external_id);