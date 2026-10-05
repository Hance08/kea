-- Monthly budgets on Expense accounts. Each row is a version that applies
-- from effective_month (YYYY-MM) until a later row for the same account.
CREATE TABLE IF NOT EXISTS budgets (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    account_id      INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    effective_month TEXT    NOT NULL CHECK (effective_month GLOB '[0-9][0-9][0-9][0-9]-[0-1][0-9]'),
    amount          INTEGER NOT NULL CHECK (amount >= 0),
    stopped         INTEGER NOT NULL DEFAULT 0 CHECK (stopped IN (0, 1)),
    CHECK (stopped = 0 OR amount = 0),
    UNIQUE (account_id, effective_month)
);

CREATE INDEX IF NOT EXISTS idx_budgets_account_month ON budgets (account_id, effective_month);
