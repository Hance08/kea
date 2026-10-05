# Store CreateAccount AccountType Validation — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add defensive AccountType validation in the store layer and a DB-level CHECK constraint so invalid type codes can never reach the accounts table.

**Architecture:** Two layers of defense: (1) a Go-level `accType.IsValid()` guard at the top of `CreateAccount`, and (2) a new migration adding `CHECK(type IN ('A','L','C','R','E'))` to the accounts table. Both are tested independently.

**Tech Stack:** Go, SQLite, golang-migrate, testify

---

### Task 1: Add `ErrInvalidAccountType` sentinel and Go-level validation in `CreateAccount`

**Files:**
- Modify: `internal/store/errors.go`
- Modify: `internal/store/sqlite_account.go:16`
- Test: `internal/store/sqlite_account_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/store/sqlite_account_test.go`:

```go
func TestCreateAccount_RejectsInvalidAccountType(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	_, err := s.CreateAccount(ctx, "Assets:Test", model.AccountType("X"), "USD", "", nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, store.ErrInvalidAccountType)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/store/ -run TestCreateAccount_RejectsInvalidAccountType -v`
Expected: FAIL — `store.ErrInvalidAccountType` is undefined.

- [ ] **Step 3: Add the sentinel error to `errors.go`**

Add a new error variable to the `var` block in `internal/store/errors.go`:

```go
ErrInvalidAccountType = fmt.Errorf("invalid account type")
```

- [ ] **Step 4: Add the validation guard to `CreateAccount`**

Insert at the very top of the `CreateAccount` method body in `internal/store/sqlite_account.go` (line 17, before the `PrepareContext` call):

```go
if !accType.IsValid() {
	return 0, fmt.Errorf("account type %q: %w", accType, ErrInvalidAccountType)
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/store/ -run TestCreateAccount_RejectsInvalidAccountType -v`
Expected: PASS

- [ ] **Step 6: Run full test suite to check for regressions**

Run: `go test ./...`
Expected: All tests pass.

- [ ] **Step 7: Commit**

```bash
git add internal/store/errors.go internal/store/sqlite_account.go internal/store/sqlite_account_test.go
git commit -m "fix: validate AccountType in store CreateAccount

Refs #28"
```

---

### Task 2: Add DB-level CHECK constraint via migration 0007

**Files:**
- Create: `migrations/0007_add_account_type_check.up.sql`
- Create: `migrations/0007_add_account_type_check.down.sql`
- Test: `internal/store/sqlite_account_test.go`

- [ ] **Step 1: Write the failing test**

This test bypasses the Go guard by inserting directly via SQL, proving the DB constraint catches bad data. Add to `internal/store/sqlite_account_test.go`:

```go
func TestAccountTypeCheckConstraint_RejectsInvalidTypeAtDBLevel(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	_, err := s.DB().ExecContext(ctx,
		`INSERT INTO accounts (name, type, currency, description) VALUES (?, ?, ?, ?)`,
		"Test:Invalid", "X", "USD", "")
	require.Error(t, err, "DB should reject invalid account type")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/store/ -run TestAccountTypeCheckConstraint_RejectsInvalidTypeAtDBLevel -v`
Expected: FAIL — the INSERT succeeds because no CHECK constraint exists yet.

- [ ] **Step 3: Create the up migration**

Create `migrations/0007_add_account_type_check.up.sql`:

```sql
-- SQLite cannot ADD CHECK to an existing table; recreate with the constraint.
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
```

- [ ] **Step 4: Create the down migration**

Create `migrations/0007_add_account_type_check.down.sql`:

```sql
-- Remove the CHECK constraint by recreating the table without it.
CREATE TABLE accounts_new (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT NOT NULL UNIQUE,
    type        TEXT NOT NULL,
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
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/store/ -run TestAccountTypeCheckConstraint_RejectsInvalidTypeAtDBLevel -v`
Expected: PASS — the INSERT now violates the CHECK constraint.

- [ ] **Step 6: Run full test suite to check for regressions**

Run: `go test ./...`
Expected: All tests pass. The table recreation preserves all data and indexes.

- [ ] **Step 7: Commit**

```bash
git add migrations/0007_add_account_type_check.up.sql migrations/0007_add_account_type_check.down.sql internal/store/sqlite_account_test.go
git commit -m "fix: add CHECK constraint for account type column

Refs #28"
```
