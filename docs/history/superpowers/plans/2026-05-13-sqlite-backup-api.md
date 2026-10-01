# SQLite Backup API Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the raw file-copy backup with SQLite's online backup API so backups are safe while the database is open (required for the web layer).

**Architecture:** Add a `DB()` accessor to `Store` so the backup package can use `mattn/go-sqlite3`'s `SQLiteConn.Backup()` via `(*sql.Conn).Raw()`. Change `backup.Run` to accept an optional `*sql.DB` for online backup when the DB is already open. The existing file-copy path stays as a fallback for the CLI startup case (backup runs before DB is opened, no `*sql.DB` available yet). The caller (`app.go`) passes the `*sql.DB` when available (web server path) or `nil` (CLI startup path).

**Tech Stack:** `github.com/mattn/go-sqlite3` (already a dependency), `database/sql`

**Issue:** [#80](https://github.com/Hance08/kea/issues/80)

---

### Task 1: Add `DB()` accessor to Store

The backup package needs access to the underlying `*sql.DB` to use SQLite's backup API. `Store.rawDB` is unexported — add a public accessor.

**Files:**
- Modify: `internal/store/sqlite.go:88` (add method after `Close()`)

- [ ] **Step 1: Write the failing test**

Create a test that calls `DB()` on a store and asserts it returns a non-nil `*sql.DB`.

```go
// in internal/store/sqlite_test.go (or a new file if sqlite_test.go doesn't exist)
func TestStore_DB_ReturnsRawDB(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	s, err := NewStore(dbPath, testMigrationsFS)
	require.NoError(t, err)
	defer s.Close()

	db := s.DB()
	require.NotNil(t, db)

	// Verify it's a working connection
	err = db.Ping()
	assert.NoError(t, err)
}
```

- [ ] **Step 2: Check for existing test infrastructure**

Look at how existing store tests set up `testMigrationsFS`. If there's already a test helper or `TestMain`, use that. If not, you'll need an empty `embed.FS` or the project's migration FS.

Run: `grep -rn "testMigration\|TestMain\|embed.FS" internal/store/ --include="*_test.go"`

Adapt the test to match whatever pattern exists.

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/store/ -run TestStore_DB -v`
Expected: compilation error — `s.DB undefined`

- [ ] **Step 4: Write minimal implementation**

Add to `internal/store/sqlite.go` after the `Close()` method:

```go
func (s *Store) DB() *sql.DB {
	return s.rawDB
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/store/ -run TestStore_DB -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/store/sqlite.go internal/store/sqlite_test.go
git commit -m "feat(store): add DB() accessor for SQLite backup API support

Exposes the underlying *sql.DB so the backup package can use
the SQLite online backup API via raw connection access."
```

---

### Task 2: Implement SQLite online backup function

Add a new function `backupOnline` to the backup package that uses `mattn/go-sqlite3`'s backup API through `(*sql.Conn).Raw()`. This produces a consistent snapshot even while the database is being written to.

**Files:**
- Create: `internal/backup/online.go`
- Create: `internal/backup/online_test.go`

- [ ] **Step 1: Write the failing test**

This test creates a real SQLite database with data, then backs it up using the online backup function and verifies the backup contains the same data.

In `internal/backup/online_test.go`:

```go
package backup

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBackupOnline_CopiesData(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "src.db")

	srcDB, err := sql.Open("sqlite3", srcPath+"?_journal_mode=WAL")
	require.NoError(t, err)
	defer srcDB.Close()

	_, err = srcDB.Exec("CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT)")
	require.NoError(t, err)
	_, err = srcDB.Exec("INSERT INTO items (name) VALUES ('alpha'), ('beta')")
	require.NoError(t, err)

	dstPath := filepath.Join(dir, "backup.db")
	err = backupOnline(context.Background(), srcDB, dstPath)
	require.NoError(t, err)

	// Verify backup file exists
	_, err = os.Stat(dstPath)
	require.NoError(t, err)

	// Verify backup contains the data
	dstDB, err := sql.Open("sqlite3", dstPath)
	require.NoError(t, err)
	defer dstDB.Close()

	var count int
	err = dstDB.QueryRow("SELECT COUNT(*) FROM items").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}

func TestBackupOnline_AtomicWithTmpFile(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "src.db")

	srcDB, err := sql.Open("sqlite3", srcPath)
	require.NoError(t, err)
	defer srcDB.Close()
	_, err = srcDB.Exec("CREATE TABLE t (id INTEGER)")
	require.NoError(t, err)

	dstPath := filepath.Join(dir, "backup.db")
	err = backupOnline(context.Background(), srcDB, dstPath)
	require.NoError(t, err)

	// No .tmp file left behind
	_, err = os.Stat(dstPath + ".tmp")
	assert.True(t, os.IsNotExist(err), "temp file should be cleaned up")
}

func TestBackupOnline_FailsOnBadDest(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "src.db")

	srcDB, err := sql.Open("sqlite3", srcPath)
	require.NoError(t, err)
	defer srcDB.Close()
	_, err = srcDB.Exec("CREATE TABLE t (id INTEGER)")
	require.NoError(t, err)

	// Destination in a non-existent directory
	dstPath := filepath.Join(dir, "no-such-dir", "backup.db")
	err = backupOnline(context.Background(), srcDB, dstPath)
	assert.Error(t, err)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/backup/ -run TestBackupOnline -v`
Expected: compilation error — `backupOnline` undefined

- [ ] **Step 3: Write implementation**

Create `internal/backup/online.go`:

```go
package backup

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	"github.com/mattn/go-sqlite3"
)

func backupOnline(ctx context.Context, srcDB *sql.DB, dstPath string) error {
	tmpPath := dstPath + ".tmp"

	dstDB, err := sql.Open("sqlite3", tmpPath)
	if err != nil {
		return fmt.Errorf("open destination: %w", err)
	}
	defer func() {
		dstDB.Close()
		if err != nil {
			os.Remove(tmpPath)
		}
	}()

	if err = dstDB.Ping(); err != nil {
		return fmt.Errorf("ping destination: %w", err)
	}

	srcConn, err := srcDB.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire source connection: %w", err)
	}
	defer srcConn.Close()

	dstConn, err := dstDB.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire destination connection: %w", err)
	}
	defer dstConn.Close()

	err = dstConn.Raw(func(dstRaw any) error {
		dstSQLiteConn, ok := dstRaw.(*sqlite3.SQLiteConn)
		if !ok {
			return fmt.Errorf("destination is not a *sqlite3.SQLiteConn")
		}

		return srcConn.Raw(func(srcRaw any) error {
			srcSQLiteConn, ok := srcRaw.(*sqlite3.SQLiteConn)
			if !ok {
				return fmt.Errorf("source is not a *sqlite3.SQLiteConn")
			}

			bk, err := dstSQLiteConn.Backup("main", srcSQLiteConn, "main")
			if err != nil {
				return fmt.Errorf("init backup: %w", err)
			}

			done, err := bk.Step(-1)
			if err != nil {
				bk.Finish()
				return fmt.Errorf("backup step: %w", err)
			}
			if !done {
				bk.Finish()
				return fmt.Errorf("backup not completed")
			}

			return bk.Finish()
		})
	})

	if err != nil {
		return err
	}

	dstDB.Close()
	if err = os.Rename(tmpPath, dstPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename backup: %w", err)
	}

	return nil
}
```

**Key details:**
- `dstSQLiteConn.Backup("main", srcSQLiteConn, "main")` — the receiver is the *destination*, second arg is the *source*. This is the `mattn/go-sqlite3` API convention.
- `Step(-1)` copies all pages in one call.
- Atomic via `.tmp` rename, matching the existing `copyFile` pattern.
- The deferred cleanup removes `.tmp` on any error.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/backup/ -run TestBackupOnline -v`
Expected: all 3 tests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/backup/online.go internal/backup/online_test.go
git commit -m "feat(backup): add SQLite online backup via backup API

Uses mattn/go-sqlite3's SQLiteConn.Backup() to produce consistent
snapshots even while the database is open and being written to.
Writes to a .tmp file and renames atomically on success."
```

---

### Task 3: Update `backup.Run` to accept optional `*sql.DB`

Change the `Run` function signature to accept an optional `*sql.DB`. When provided, use `backupOnline` instead of `copyFile`. When `nil`, fall back to the existing `copyFile` (CLI startup path where DB isn't open yet).

**Files:**
- Modify: `internal/backup/backup.go:27-28` (change `Run`/`run` signatures)
- Modify: `internal/backup/backup_test.go` (update all `run()` calls)
- Modify: `internal/app/app.go:34` (update caller)

- [ ] **Step 1: Write the failing test for online backup path**

Add to `internal/backup/backup_test.go`:

```go
func TestRun_WithDB_UsesOnlineBackup(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "kea.db")

	// Create a real SQLite database with data
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL")
	require.NoError(t, err)
	defer db.Close()

	_, err = db.Exec("CREATE TABLE test (id INTEGER PRIMARY KEY, val TEXT)")
	require.NoError(t, err)
	_, err = db.Exec("INSERT INTO test (val) VALUES ('hello')")
	require.NoError(t, err)

	clk := fakeClock{t: fixedTime("2026-04-14")}
	err = run(dbPath, clk, db)
	require.NoError(t, err)

	// Verify backup file contains actual database data
	backupDir := filepath.Join(dir, "backups")
	backupPath := filepath.Join(backupDir, "kea_daily_2026-04-14.db")
	backupDB, err := sql.Open("sqlite3", backupPath)
	require.NoError(t, err)
	defer backupDB.Close()

	var val string
	err = backupDB.QueryRow("SELECT val FROM test WHERE id = 1").Scan(&val)
	require.NoError(t, err)
	assert.Equal(t, "hello", val)
}
```

Add the import for `"database/sql"` and `_ "github.com/mattn/go-sqlite3"` to the test file's imports.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/backup/ -run TestRun_WithDB -v`
Expected: compilation error — `run` takes wrong number of arguments

- [ ] **Step 3: Update `run` signature and dispatch logic**

In `internal/backup/backup.go`, change the `Run` and `run` functions:

```go
func Run(dbPath string, db *sql.DB) error {
	return run(dbPath, realClock{}, db)
}

func run(dbPath string, clk clock, db *sql.DB) error {
```

Add `"context"` and `"database/sql"` to the imports.

Then change the copy call inside the tier loop (line 54) from:

```go
if err := copyFile(dbPath, dst); err != nil {
```

to:

```go
if err := doBackup(dbPath, dst, db); err != nil {
```

And add the dispatch function:

```go
func doBackup(dbPath, dst string, db *sql.DB) error {
	if db != nil {
		return backupOnline(context.Background(), db, dst)
	}
	return copyFile(dbPath, dst)
}
```

- [ ] **Step 4: Update all existing `run()` calls in tests**

In `internal/backup/backup_test.go`, every existing call to `run(dbPath, clk)` needs a third argument `nil`:

Find and replace all occurrences:
- `run(dbPath, fakeClock{...})` → `run(dbPath, fakeClock{...}, nil)`

There are calls in these test functions:
- `TestRun_NoDBFile`
- `TestRun_FirstRun_AllThreeTiersCreated`
- `TestRun_AlreadyCurrent_NoNewFiles`
- `TestRun_DailyDue_OtherTiersCurrent`
- `TestRun_WeeklyDue_OtherTiersCurrent`
- `TestRun_MonthlyDue_OtherTiersCurrent`
- `TestRun_RotationTriggered`
- `TestRun_CopyFailure_ReturnsError`

- [ ] **Step 5: Update caller in `app.go`**

In `internal/app/app.go:34`, change:

```go
if err := backup.Run(dbPathRaw); err != nil {
```

to:

```go
if err := backup.Run(dbPathRaw, nil); err != nil {
```

The CLI startup path passes `nil` because the DB isn't open yet. The web server path (future `kea serve`) will pass the `*sql.DB` after opening the store.

- [ ] **Step 6: Run all tests**

Run: `go test ./internal/backup/ -v`
Expected: all tests PASS (existing + new)

Run: `go test ./...`
Expected: all tests PASS (verify `app.go` compiles)

- [ ] **Step 7: Commit**

```bash
git add internal/backup/backup.go internal/backup/online.go internal/backup/backup_test.go internal/app/app.go
git commit -m "feat(backup): use SQLite backup API when DB is open

backup.Run now accepts an optional *sql.DB. When provided, it uses
the SQLite online backup API for consistent snapshots. When nil,
it falls back to file copy (CLI startup path).

Closes #80"
```

---

### Task 4: Add test for backup during concurrent writes

Verify the online backup produces a consistent snapshot even while a write transaction is in progress. This is the core scenario from the issue.

**Files:**
- Modify: `internal/backup/online_test.go` (add concurrency test)

- [ ] **Step 1: Write the concurrent-write test**

Add to `internal/backup/online_test.go`:

```go
func TestBackupOnline_ConsistentDuringWrites(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "src.db")

	srcDB, err := sql.Open("sqlite3", srcPath+"?_journal_mode=WAL&_busy_timeout=5000")
	require.NoError(t, err)
	defer srcDB.Close()

	_, err = srcDB.Exec("CREATE TABLE counter (id INTEGER PRIMARY KEY, n INTEGER)")
	require.NoError(t, err)
	_, err = srcDB.Exec("INSERT INTO counter (n) VALUES (0)")
	require.NoError(t, err)

	// Start a goroutine that writes continuously
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				srcDB.Exec("UPDATE counter SET n = n + 1 WHERE id = 1")
			}
		}
	}()

	// Perform backup while writes are happening
	dstPath := filepath.Join(dir, "backup.db")
	err = backupOnline(context.Background(), srcDB, dstPath)
	cancel()
	<-done
	require.NoError(t, err)

	// Verify the backup is a valid, openable database
	dstDB, err := sql.Open("sqlite3", dstPath)
	require.NoError(t, err)
	defer dstDB.Close()

	var n int
	err = dstDB.QueryRow("SELECT n FROM counter WHERE id = 1").Scan(&n)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, n, 0, "counter should have a valid value")

	// Run integrity check on the backup
	var result string
	err = dstDB.QueryRow("PRAGMA integrity_check").Scan(&result)
	require.NoError(t, err)
	assert.Equal(t, "ok", result)
}
```

- [ ] **Step 2: Run test to verify it passes**

Run: `go test ./internal/backup/ -run TestBackupOnline_ConsistentDuringWrites -v -count=1`
Expected: PASS — the backup API guarantees a consistent snapshot

- [ ] **Step 3: Commit**

```bash
git add internal/backup/online_test.go
git commit -m "test(backup): verify online backup consistency during concurrent writes

Runs backup while a goroutine continuously updates the database,
then verifies the backup passes PRAGMA integrity_check."
```

---

## Summary of Changes

| File | Change |
|------|--------|
| `internal/store/sqlite.go` | Add `DB()` accessor method |
| `internal/backup/online.go` | New — `backupOnline()` using SQLite backup API |
| `internal/backup/online_test.go` | New — tests for online backup (basic + concurrent writes) |
| `internal/backup/backup.go` | `Run`/`run` accept optional `*sql.DB`, dispatch to `backupOnline` or `copyFile` |
| `internal/backup/backup_test.go` | Update existing calls, add `TestRun_WithDB_UsesOnlineBackup` |
| `internal/app/app.go` | Pass `nil` to `backup.Run` (CLI startup path) |

## Notes for Future Web Server Integration

When `kea serve` is implemented, the web server startup path should:

```go
// After opening the store
store, err := store.NewStore(dbPath, migrationFS)
// ...
backup.Run(dbPath, store.DB())
```

This gives the backup package a live `*sql.DB` so it uses the online backup API instead of file copy.
