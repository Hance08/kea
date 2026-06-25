PRAGMA foreign_keys = OFF;

BEGIN;

CREATE TABLE transactions_old (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp   INTEGER NOT NULL,
    description TEXT,
    status      INTEGER NOT NULL DEFAULT 0,
    external_id TEXT,
    type        TEXT NOT NULL DEFAULT ''
);

INSERT INTO transactions_old (id, timestamp, description, status, external_id, type)
SELECT id, timestamp, description, status, external_id, type
FROM transactions;

DROP TABLE transactions;
ALTER TABLE transactions_old RENAME TO transactions;

CREATE INDEX IF NOT EXISTS idx_transactions_timestamp ON transactions (timestamp);
CREATE UNIQUE INDEX IF NOT EXISTS idx_transactions_external_id ON transactions(external_id);

COMMIT;

PRAGMA foreign_keys = ON;
