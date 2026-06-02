-- Composite index for GetUnreconciledTransactionsByAccount, which filters on
-- (account_id, reconciled). With separate single-column indexes SQLite can
-- only use one per scan; the composite lets it satisfy the full predicate.
CREATE INDEX IF NOT EXISTS idx_splits_account_reconciled
    ON splits (account_id, reconciled);
