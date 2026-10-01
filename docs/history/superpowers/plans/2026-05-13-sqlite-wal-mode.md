# SQLite WAL Mode & Connection Configuration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enable WAL journal mode, busy timeout, and connection pool limits on the SQLite database so the CLI and future web server can safely access the same database file concurrently.

**Architecture:** The fix is entirely within `internal/store/sqlite.go:NewStore`. We add `_journal_mode=WAL` and `_busy_timeout=5000` to the DSN query string, then call `db.SetMaxOpenConns(1)` after opening to serialize writes through Go's connection pool. A new integration test verifies the pragmas are active on every database opened by the store.

**Tech Stack:** Go, `database/sql`, `github.com/mattn/go-sqlite3`, `testify`

**Closes:** [#75](https://github.com/Hance08/kea/issues/75)

---

### Task 1: Write failing test for WAL mode and busy timeout

**Files:**
- Create: `internal/store/sqlite_pragma_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/store/sqlite_pragma_test.go`:

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package store_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewStore_EnablesWALMode(t *testing.T) {
	s := setupTestDB(t)

	var journalMode string
	err := s.QueryRowContext(context.Background(), "PRAGMA journal_mode").Scan(&journalMode)
	require.NoError(t, err)
	assert.Equal(t, "wal", journalMode)
}

func TestNewStore_SetsBusyTimeout(t *testing.T) {
	s := setupTestDB(t)

	var timeout int
	err := s.QueryRowContext(context.Background(), "PRAGMA busy_timeout").Scan(&timeout)
	require.NoError(t, err)
	assert.Equal(t, 5000, timeout)
}
```

- [ ] **Step 2: Check if `QueryRowContext` is accessible on `*Store`**

`Store.db` is unexported and `Store` does not expose `QueryRowContext` directly. The test needs a way to run raw pragma queries. We have two options:

1. Add a `QueryRowContext` method to `Store` that delegates to `db`.
2. Use `Store.ExecTx` to run the query inside a transaction.

Option 1 is cleaner because pragmas don't need transactions. Add a small exported method.

- [ ] **Step 3: Add `QueryRowContext` to Store for test access**

Modify `internal/store/sqlite.go` — add after the `Close()` method:

```go
func (s *Store) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return s.db.QueryRowContext(ctx, query, args...)
}
```

- [ ] **Step 4: Run tests to verify they fail**

Run: `go test ./internal/store/ -run "TestNewStore_Enables|TestNewStore_Sets" -v`

Expected: `TestNewStore_EnablesWALMode` fails (gets `"delete"` instead of `"wal"`), `TestNewStore_SetsBusyTimeout` fails (gets `0` instead of `5000`).

---

### Task 2: Implement WAL mode, busy timeout, and connection pool config

**Files:**
- Modify: `internal/store/sqlite.go:40`

- [ ] **Step 1: Update the DSN and add pool config**

In `internal/store/sqlite.go`, replace line 40:

```go
db, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=on")
```

with:

```go
db, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=on&_journal_mode=WAL&_busy_timeout=5000")
```

Then add connection pool configuration right after the `defer` block (after line 46, before the `if err != nil` check):

```go
db.SetMaxOpenConns(1)
```

The final `NewStore` function should look like:

```go
func NewStore(dbPath string, migrationsFS fs.FS) (*Store, error) {
	dbDir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return nil, fmt.Errorf("can not create database directory %s: %w", dbDir, err)
	}

	db, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=on&_journal_mode=WAL&_busy_timeout=5000")
	success := false
	defer func() {
		if !success {
			_ = db.Close()
		}
	}()

	if err != nil {
		return nil, fmt.Errorf("can not open database : %w", err)
	}

	db.SetMaxOpenConns(1)

	if err = db.Ping(); err != nil {
		return nil, fmt.Errorf("can not connect with database : %w", err)
	}
	if err := runMigrations(db, migrationsFS); err != nil {
		return nil, fmt.Errorf("failed to migrate database : %w", err)
	}

	success = true
	return &Store{db: db, rawDB: db}, nil
}
```

Note: `SetMaxOpenConns(1)` is placed after the error check for `sql.Open` but before `Ping`, so the pool is configured before any real connection is made.

- [ ] **Step 2: Run the new pragma tests**

Run: `go test ./internal/store/ -run "TestNewStore_Enables|TestNewStore_Sets" -v`

Expected: Both PASS.

- [ ] **Step 3: Run the full test suite to check for regressions**

Run: `go test ./...`

Expected: All tests pass. `SetMaxOpenConns(1)` serializes connections, which is safe since all existing tests are single-goroutine.

- [ ] **Step 4: Commit**

```bash
git add internal/store/sqlite.go internal/store/sqlite_pragma_test.go
git commit -m "fix: enable WAL mode, busy timeout, and connection pool limit for SQLite

Add _journal_mode=WAL and _busy_timeout=5000 to the DSN so readers
and writers can operate concurrently without 'database is locked'
errors. Set MaxOpenConns(1) to serialize writes through Go's pool.

Add QueryRowContext to Store for pragma verification in tests.

Closes #75"
```

---

### Task 3: Verify existing store tests still pass with WAL

This task is a verification checkpoint — no code changes.

- [ ] **Step 1: Run all store tests**

Run: `go test ./internal/store/... -v`

Expected: All tests pass, including the rename and error sentinel tests.

- [ ] **Step 2: Run the full project test suite**

Run: `go test ./...`

Expected: All packages pass. No regressions from WAL mode or single-connection pool.

- [ ] **Step 3: Build the binary**

Run: `make build`

Expected: Builds successfully with no compilation errors.
