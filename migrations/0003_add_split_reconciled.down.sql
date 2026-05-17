DROP INDEX IF EXISTS idx_splits_reconciled;

CREATE TABLE splits_new (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    transaction_id   INTEGER NOT NULL,
    account_id       INTEGER NOT NULL,
    amount           INTEGER NOT NULL,
    currency         TEXT NOT NULL,
    memo             TEXT,
    FOREIGN KEY (transaction_id) REFERENCES transactions(id) ON DELETE CASCADE,
    FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE RESTRICT
);

INSERT INTO splits_new SELECT id, transaction_id, account_id, amount, currency, memo FROM splits;
DROP TABLE splits;
ALTER TABLE splits_new RENAME TO splits;

CREATE INDEX IF NOT EXISTS idx_splits_transaction_id ON splits (transaction_id);
CREATE INDEX IF NOT EXISTS idx_splits_account_id ON splits (account_id);
