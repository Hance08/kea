PRAGMA foreign_keys = OFF;

BEGIN;

CREATE TABLE transactions_new (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp   INTEGER NOT NULL,
    description TEXT,
    status      INTEGER NOT NULL DEFAULT 0,
    external_id TEXT,
    type        TEXT NOT NULL DEFAULT '',
    regular     INTEGER,
    CHECK (
        (type IN ('Income', 'Expense') AND regular IS NOT NULL AND regular IN (0, 1))
        OR (type NOT IN ('Income', 'Expense') AND regular IS NULL)
    )
);

INSERT INTO transactions_new (id, timestamp, description, status, external_id, type, regular)
SELECT id, timestamp, description, status, external_id, type,
       CASE WHEN type IN ('Income', 'Expense') THEN 1 ELSE NULL END
FROM transactions;

DROP TABLE transactions;
ALTER TABLE transactions_new RENAME TO transactions;

CREATE INDEX IF NOT EXISTS idx_transactions_timestamp ON transactions (timestamp);
CREATE UNIQUE INDEX IF NOT EXISTS idx_transactions_external_id ON transactions(external_id);

COMMIT;

PRAGMA foreign_keys = ON;
