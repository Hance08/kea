# Missing Down Migrations Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `.down.sql` files for migrations 0001–0004 so the schema can be fully rolled back with `golang-migrate`.

**Architecture:** Each down migration inverts its corresponding up migration. Migration 0001 and 0004 create tables, so their downs drop them. Migrations 0002 and 0003 add columns, so their downs use SQLite's "recreate table without column" pattern (already established in `0005_add_transaction_type.down.sql`).

**Tech Stack:** SQLite, golang-migrate

**Closes:** #30, #59

---

## File Structure

- Create: `migrations/0001_create_initial_schema.down.sql`
- Create: `migrations/0002_add_external_id.down.sql`
- Create: `migrations/0003_add_split_reconciled.down.sql`
- Create: `migrations/0004_add_account_reconcile_state.down.sql`

---

### Task 1: Down migration for 0004 (account_reconcile_state)

**Files:**
- Create: `migrations/0004_add_account_reconcile_state.down.sql`

This is the simplest — the up creates a table, the down drops it.

- [ ] **Step 1: Create the down migration**

```sql
DROP TABLE IF EXISTS account_reconcile_state;
```

- [ ] **Step 2: Verify round-trip**

```bash
cd /Users/hance/programming/kea
# Apply up to 0004, then roll back to 0003, then re-apply 0004
# Uses the test binary or a temp DB
sqlite3 :memory: < migrations/0001_create_initial_schema.up.sql
```

We'll do a full round-trip test in Task 5.

- [ ] **Step 3: Commit**

```bash
git add migrations/0004_add_account_reconcile_state.down.sql
git commit -m "fix: add down migration for 0004 (account_reconcile_state)"
```

---

### Task 2: Down migration for 0003 (split reconciled column)

**Files:**
- Create: `migrations/0003_add_split_reconciled.down.sql`

The up adds column `reconciled` and an index. The down recreates splits without that column (SQLite lacks DROP COLUMN in older versions). At this migration point, the splits schema is: id, transaction_id, account_id, amount, currency, memo, reconciled.

- [ ] **Step 1: Create the down migration**

```sql
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
```

- [ ] **Step 2: Commit**

```bash
git add migrations/0003_add_split_reconciled.down.sql
git commit -m "fix: add down migration for 0003 (split reconciled column)"
```

---

### Task 3: Down migration for 0002 (external_id column)

**Files:**
- Create: `migrations/0002_add_external_id.down.sql`

The up adds column `external_id` and a unique index on transactions. At this migration point, the transactions schema is: id, timestamp, description, status, external_id.

- [ ] **Step 1: Create the down migration**

```sql
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
```

- [ ] **Step 2: Commit**

```bash
git add migrations/0002_add_external_id.down.sql
git commit -m "fix: add down migration for 0002 (external_id column)"
```

---

### Task 4: Down migration for 0001 (initial schema)

**Files:**
- Create: `migrations/0001_create_initial_schema.down.sql`

The up creates tables accounts, transactions, splits (plus indexes). The down drops them in FK-safe order: splits first (references both others), then transactions, then accounts.

- [ ] **Step 1: Create the down migration**

```sql
DROP TABLE IF EXISTS splits;
DROP TABLE IF EXISTS transactions;
DROP TABLE IF EXISTS accounts;
```

- [ ] **Step 2: Commit**

```bash
git add migrations/0001_create_initial_schema.down.sql
git commit -m "fix: add down migration for 0001 (initial schema)"
```

---

### Task 5: Round-trip verification

Verify up→down→up produces a clean schema for all four migrations.

- [ ] **Step 1: Write a shell script to verify round-trip**

```bash
DB=$(mktemp)
# Apply all ups in order
for i in 0001 0002 0003 0004; do
  sqlite3 "$DB" < "migrations/${i}_"*".up.sql"
done
# Roll back all downs in reverse
for i in 0004 0003 0002 0001; do
  sqlite3 "$DB" < "migrations/${i}_"*".down.sql"
done
# Re-apply all ups
for i in 0001 0002 0003 0004; do
  sqlite3 "$DB" < "migrations/${i}_"*".up.sql"
done
# Verify tables exist
sqlite3 "$DB" ".tables" | grep -q accounts && echo "PASS" || echo "FAIL"
rm "$DB"
```

Expected: PASS — schema is intact after full round-trip.

- [ ] **Step 2: Run existing tests**

```bash
go test ./...
```

Expected: all tests pass (down migrations don't affect runtime behavior).

- [ ] **Step 3: Final commit (if any adjustments needed)**

Only if the round-trip test revealed issues that required fixes.

---

## Notes

- The "recreate table" pattern for 0002 and 0003 drops and recreates indexes because SQLite drops indexes when the original table is dropped.
- `IF EXISTS` / `IF NOT EXISTS` guards make the downs idempotent.
- Running `up→down→up` is the canonical correctness check for golang-migrate files.
