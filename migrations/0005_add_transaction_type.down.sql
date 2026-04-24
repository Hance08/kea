-- SQLite does not support DROP COLUMN in older versions; recreate without the column.
CREATE TABLE transactions_new (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp   INTEGER NOT NULL,
    description TEXT,
    status      INTEGER NOT NULL DEFAULT 0,
    external_id TEXT UNIQUE
);

INSERT INTO transactions_new SELECT id, timestamp, description, status, external_id FROM transactions;
DROP TABLE transactions;
ALTER TABLE transactions_new RENAME TO transactions;

CREATE INDEX IF NOT EXISTS idx_transactions_timestamp ON transactions (timestamp);
CREATE UNIQUE INDEX IF NOT EXISTS idx_transactions_external_id ON transactions (external_id);
