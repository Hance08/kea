DROP INDEX IF EXISTS idx_transactions_external_id;

CREATE TABLE transactions_new (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp   INTEGER NOT NULL,
    description TEXT,
    status      INTEGER NOT NULL DEFAULT 0
);

INSERT INTO transactions_new SELECT id, timestamp, description, status FROM transactions;
DROP TABLE transactions;
ALTER TABLE transactions_new RENAME TO transactions;

CREATE INDEX IF NOT EXISTS idx_transactions_timestamp ON transactions (timestamp);
