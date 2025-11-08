-- This is the first migration file for creating core tables
-- "IF NOT EXISTS" ensure that this file can re-execute safely

-- 1. Accounts Table
-- Save A, L, C, R, E five main types of its account
CREATE TABLE IF NOT EXISTS accounts (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT NOT NULL UNIQUE,          -- full path name, e.g., "Assets:Bank:TaiShinBank"
    type        TEXT NOT NULL,                 -- account type expected value -> (A, L, C, R, E)
    parent_id   INTEGER,                       -- point to accounts.id, is use to create subaccount
    currency    TEXT NOT NULL,   -- set up currency
    description TEXT,                          -- for TUI to display readable name
    is_hidden   INTEGER NOT NULL DEFAULT 0,    -- 0=false, 1=true (is used to archive account)
    
    -- setting foreign key, ensure that parent_id must point at another valid accounts.id
    FOREIGN KEY (parent_id) REFERENCES accounts(id)
);

-- create index for column "name", this will let the query "WHERE name = ..." faster
CREATE INDEX IF NOT EXISTS idx_accounts_name ON accounts (name);


-- 2. Transactions Table
-- record one transaction, such like "Buy a cup of coffee"
CREATE TABLE IF NOT EXISTS transactions (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp   INTEGER NOT NULL,              -- date of the transaction occurred (Unix timestamp)
    description TEXT,                          -- description
    status      INTEGER NOT NULL DEFAULT 0,    -- 0=Pending, 1=Cleared
);

-- create index for column "timestamp"
CREATE INDEX IF NOT EXISTS idx_transactions_timestamp ON transactions (timestamp);


-- 3. Splits Table
-- core of double-entry system, link the transaction to the account
CREATE TABLE IF NOT EXISTS splits (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    transaction_id   INTEGER NOT NULL,  -- point to transactions.id
    account_id       INTEGER NOT NULL,  -- point to accounts.id
    amount           INTEGER NOT NULL,  -- store in cent, such like NT$150 -> 15000
    currency         TEXT NOT NULL,
    memo             TEXT,              -- 

    -- key of database constraint
    -- ON DELETE CASCADE: if transactions is deleted, it will auto delete all the related splits
    FOREIGN KEY (transaction_id) REFERENCES transactions(id) ON DELETE CASCADE,
    
    -- ON DELETE RESTRICT: if accounts still have splits references, prohibit to delete this account
    FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE RESTRICT
);

-- create two indexes for transaction_id and account_id 
CREATE INDEX IF NOT EXISTS idx_splits_transaction_id ON splits (transaction_id);
CREATE INDEX IF NOT EXISTS idx_splits_account_id ON splits (account_id);