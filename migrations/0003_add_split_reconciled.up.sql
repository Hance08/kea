-- Track reconciliation at the split level so that multi-account transactions
-- (transfers, split expenses) can be reconciled independently per account.
ALTER TABLE splits ADD COLUMN reconciled INTEGER NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_splits_reconciled ON splits (reconciled);
