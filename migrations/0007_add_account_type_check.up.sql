PRAGMA foreign_keys = OFF;

CREATE TABLE accounts_new (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT NOT NULL UNIQUE,
    type        TEXT NOT NULL CHECK(type IN ('A','L','C','R','E')),
    parent_id   INTEGER,
    currency    TEXT NOT NULL,
    description TEXT,
    is_hidden   INTEGER NOT NULL DEFAULT 0,

    FOREIGN KEY (parent_id) REFERENCES accounts(id)
);

INSERT INTO accounts_new SELECT id, name, type, parent_id, currency, description, is_hidden FROM accounts;

DROP TABLE accounts;
ALTER TABLE accounts_new RENAME TO accounts;

CREATE INDEX IF NOT EXISTS idx_accounts_name ON accounts (name);

PRAGMA foreign_keys = ON;
