# Fix Backup WAL Safety (Issue #111) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `backup.Run` safe when another process (web server) has the DB open in WAL mode, by always using the SQLite online backup API instead of raw file copy.

**Architecture:** When `db == nil` is passed to `doBackup`, instead of falling back to an unsafe `copyFile`, open a temporary `*sql.DB` connection and use the existing `backupOnline` function. This provides a consistent snapshot via SQLite's backup API regardless of concurrent writers. Remove the now-dead `copyFile` function.

**Tech Stack:** Go, SQLite (`mattn/go-sqlite3`), `database/sql`

---

### Task 1: Update `mkDB` test helper to create real SQLite databases

The existing `mkDB` helper writes raw bytes (`"fake-db"`) which isn't a valid SQLite database. After the fix, the `db=nil` path will open the file with `sql.Open`, so all tests need real DB files.

**Files:**
- Modify: `internal/backup/backup_test.go:173-176` (`mkDB` function)

- [ ] **Step 1: Update `mkDB` to create a real SQLite database**

Replace the current `mkDB`:

```go
func mkDB(t *testing.T, dbPath string) {
	t.Helper()
	db, err := sql.Open("sqlite3", dbPath)
	require.NoError(t, err)
	_, err = db.Exec("CREATE TABLE _backup_marker (id INTEGER PRIMARY KEY)")
	require.NoError(t, err)
	require.NoError(t, db.Close())
}
```

- [ ] **Step 2: Run existing tests to verify they still pass**

Run: `go test ./internal/backup/ -v -count=1`
Expected: All existing tests pass (they use `mkDB` to create the DB, then `run(dbPath, clk, nil)` which currently uses `copyFile` — the real SQLite file is still a valid file to copy).

- [ ] **Step 3: Commit**

```bash
git add internal/backup/backup_test.go
git commit -m "test: make mkDB create real SQLite databases for backup tests (#111)"
```

---

### Task 2: Write failing test for `db=nil` WAL-safety

Add a test that creates a real SQLite DB in WAL mode, runs `doBackup` with `db=nil`, and verifies the backup is a valid, queryable SQLite database. This test will initially fail because `copyFile` produces a valid copy but doesn't guarantee consistency — once we verify the test structure works, we'll use it to validate the online-backup switch.

**Files:**
- Modify: `internal/backup/backup_test.go`

- [ ] **Step 1: Write the test**

Add after the existing `TestRun_WithDB_UsesOnlineBackup` test:

```go
func TestDoBackup_NilDB_ProducesConsistentSnapshot(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "kea.db")

	srcDB, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	require.NoError(t, err)

	_, err = srcDB.Exec("CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT)")
	require.NoError(t, err)
	_, err = srcDB.Exec("INSERT INTO items (name) VALUES ('alpha'), ('beta')")
	require.NoError(t, err)

	// Leave data in WAL by NOT closing or checkpointing.
	// Write to WAL then keep the connection open to simulate a running web server.
	_, err = srcDB.Exec("INSERT INTO items (name) VALUES ('gamma')")
	require.NoError(t, err)
	defer srcDB.Close()

	dst := filepath.Join(dir, "backup.db")
	err = doBackup(dbPath, dst, nil)
	require.NoError(t, err)

	backupDB, err := sql.Open("sqlite3", dst)
	require.NoError(t, err)
	defer backupDB.Close()

	var result string
	err = backupDB.QueryRow("PRAGMA integrity_check").Scan(&result)
	require.NoError(t, err)
	assert.Equal(t, "ok", result)

	var count int
	err = backupDB.QueryRow("SELECT COUNT(*) FROM items").Scan(&count)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, count, 2, "backup should contain at least the committed rows")
}
```

- [ ] **Step 2: Run the test to confirm it exercises the nil-db path**

Run: `go test ./internal/backup/ -v -run TestDoBackup_NilDB_ProducesConsistentSnapshot -count=1`
Expected: The test passes currently (copyFile happens to work for this case since we're on the same machine), but after the implementation change it will use the online backup API instead. The test validates the backup is a valid SQLite DB either way.

- [ ] **Step 3: Commit**

```bash
git add internal/backup/backup_test.go
git commit -m "test: add WAL-safety test for nil-db backup path (#111)"
```

---

### Task 3: Implement the fix — use online backup when `db == nil`

Change `doBackup` to open a temporary `*sql.DB` connection and use `backupOnline` instead of falling back to `copyFile`.

**Files:**
- Modify: `internal/backup/backup.go:131-139` (`doBackup` function)

- [ ] **Step 1: Update `doBackup` to open a temp connection when `db` is nil**

Replace the `doBackup` function:

```go
func doBackup(dbPath, dst string, db *sql.DB) error {
	if db != nil {
		return backupOnline(context.Background(), db, dst)
	}

	tmpDB, err := sql.Open("sqlite3", dbPath+"?_busy_timeout=5000")
	if err != nil {
		return fmt.Errorf("open for backup: %w", err)
	}
	defer tmpDB.Close()

	if err := tmpDB.Ping(); err != nil {
		return fmt.Errorf("ping for backup: %w", err)
	}

	return backupOnline(context.Background(), tmpDB, dst)
}
```

- [ ] **Step 2: Run all backup tests**

Run: `go test ./internal/backup/ -v -count=1`
Expected: ALL tests pass, including the new `TestDoBackup_NilDB_ProducesConsistentSnapshot`.

- [ ] **Step 3: Commit**

```bash
git add internal/backup/backup.go
git commit -m "fix: use online backup API when db is nil for WAL safety (#111)"
```

---

### Task 4: Remove dead `copyFile` code and its tests

After the fix, `copyFile` is unused. Remove it and its dedicated tests.

**Files:**
- Modify: `internal/backup/backup.go` (remove `copyFile` function, lines 141-174)
- Modify: `internal/backup/backup_test.go` (remove `TestCopyFile` and `TestCopyFile_LeavesNoTmpOnFailure`)

- [ ] **Step 1: Remove `copyFile` from `backup.go`**

Delete the `copyFile` function (lines 141-174) and its doc comment (line 141-142). Also remove the `"io"` import since it's only used by `copyFile`.

- [ ] **Step 2: Remove `copyFile` tests from `backup_test.go`**

Delete `TestCopyFile` (lines 70-84) and `TestCopyFile_LeavesNoTmpOnFailure` (lines 86-99).

- [ ] **Step 3: Run all backup tests**

Run: `go test ./internal/backup/ -v -count=1`
Expected: All remaining tests pass. No compilation errors from removed code.

- [ ] **Step 4: Run full test suite**

Run: `go test ./... -count=1`
Expected: All tests pass across the entire project.

- [ ] **Step 5: Commit**

```bash
git add internal/backup/backup.go internal/backup/backup_test.go
git commit -m "refactor: remove dead copyFile function after WAL-safety fix (#111)"
```

---

### Task 5: Update `doBackup` doc comment and `Run` doc comment

The doc comments reference `copyFile` and the "nil means direct copy" behavior. Update them to reflect the new behavior.

**Files:**
- Modify: `internal/backup/backup.go` (doc comments on `Run` and `doBackup`)

- [ ] **Step 1: Update the `Run` doc comment**

Replace the `Run` doc comment (lines 28-31):

```go
// Run backs up dbPath if any tier is due. It is a no-op when dbPath does not
// exist. When db is non-nil, that connection is reused; when nil, a temporary
// connection is opened. Both paths use the SQLite online backup API for a
// consistent snapshot safe under concurrent access.
```

- [ ] **Step 2: Update the `doBackup` doc comment**

Replace the `doBackup` doc comment:

```go
// doBackup creates a backup using the SQLite online backup API. When db is
// non-nil the existing connection is used; otherwise a temporary connection
// is opened and closed after the backup completes.
```

- [ ] **Step 3: Run tests one final time**

Run: `go test ./internal/backup/ -v -count=1`
Expected: All tests pass.

- [ ] **Step 4: Commit**

```bash
git add internal/backup/backup.go
git commit -m "docs: update backup doc comments to reflect WAL-safe behavior (#111)"
```
