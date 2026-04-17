-- Per-account reconciliation state. Stores the running reconciled balance so
-- that the reconcile UI can use the bank statement's actual closing balance
-- instead of asking for a net-change delta on every session.
CREATE TABLE IF NOT EXISTS account_reconcile_state (
    account_id              INTEGER PRIMARY KEY,
    last_reconciled_balance INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE CASCADE
);
