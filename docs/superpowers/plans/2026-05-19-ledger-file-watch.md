# Ledger File-Watch Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enable the Registry to watch `ledgers.yaml` for changes and notify subscribers so the Store can gracefully swap its DB connection — fixing the split-brain between CLI and long-lived processes.

**Architecture:** Registry owns an fsnotify watcher on `ledgers.yaml` and fires `OnSwitch` callbacks when the active ledger changes. Store protects `db`/`rawDB` with a `sync.RWMutex` and exposes a `Swap` method. App wires the two together via a callback.

**Tech Stack:** Go, `github.com/fsnotify/fsnotify`, `sync.RWMutex`, SQLite

---

## File Structure

| File | Responsibility |
|------|---------------|
| `internal/ledger/registry.go` | Add `mu`, `callbacks`, `watcher` fields; `OnSwitch`, `Watch`, `StopWatch`, `reload` methods |
| `internal/ledger/registry_test.go` | Add watcher + callback tests |
| `internal/store/sqlite.go` | Add `mu sync.RWMutex`; `Swap` method; read-lock in `ExecTx`, `Close`, `DB` |
| `internal/store/export_test.go` | Update `QueryRowContext` to acquire read-lock |
| `internal/store/sqlite_account.go` | Add read-lock to all 13 public methods |
| `internal/store/sqlite_transaction.go` | Add read-lock to all 17 public methods |
| `internal/store/sqlite_reconcile.go` | Add read-lock to all 5 public methods |
| `internal/store/sqlite_swap_test.go` | New: swap + concurrency tests |
| `internal/app/app.go` | Add `Registry` field; update `NewApp` signature; wire callback; update cleanup |
| `cmd/root.go` | Pass `registry` to `NewApp` |
| `go.mod` / `go.sum` | Add `github.com/fsnotify/fsnotify` |

---

### Task 1: Add fsnotify dependency

**Files:**
- Modify: `go.mod`

- [ ] **Step 1: Add the dependency**

Run:
```bash
go get github.com/fsnotify/fsnotify
```

- [ ] **Step 2: Tidy**

Run:
```bash
go mod tidy
```

- [ ] **Step 3: Verify the project compiles**

Run:
```bash
go build ./...
```
Expected: success, no errors.

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum
git commit -m "chore: add fsnotify dependency for ledger file-watch (#119)"
```

---

### Task 2: Registry — OnSwitch, Watch, StopWatch

**Files:**
- Modify: `internal/ledger/registry.go`
- Modify: `internal/ledger/registry_test.go`

- [ ] **Step 1: Write the failing tests**

Add these tests to `internal/ledger/registry_test.go`:

```go
func TestWatch_FiresOnSwitch(t *testing.T) {
	dir := t.TempDir()
	r, err := Load(dir)
	require.NoError(t, err)
	require.NoError(t, r.Add("work", "/tmp/work.db"))

	var gotName, gotPath string
	done := make(chan struct{})
	r.OnSwitch(func(name, path string) {
		gotName = name
		gotPath = path
		close(done)
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = r.Watch(ctx)
	}()

	// Give the watcher time to start.
	time.Sleep(200 * time.Millisecond)

	// Simulate an external process switching the ledger.
	r2, err := Load(dir)
	require.NoError(t, err)
	require.NoError(t, r2.Switch("work"))

	select {
	case <-done:
		assert.Equal(t, "work", gotName)
		assert.Equal(t, "/tmp/work.db", gotPath)
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for OnSwitch callback")
	}
}

func TestWatch_NoCallbackWhenActiveUnchanged(t *testing.T) {
	dir := t.TempDir()
	r, err := Load(dir)
	require.NoError(t, err)
	require.NoError(t, r.Add("work", "/tmp/work.db"))

	called := make(chan struct{}, 1)
	r.OnSwitch(func(name, path string) {
		called <- struct{}{}
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = r.Watch(ctx)
	}()

	time.Sleep(200 * time.Millisecond)

	// Write the same active ledger — should NOT fire callback.
	r2, err := Load(dir)
	require.NoError(t, err)
	require.NoError(t, r2.Switch("default"))

	select {
	case <-called:
		t.Fatal("callback should not fire when active ledger is unchanged")
	case <-time.After(500 * time.Millisecond):
		// OK — no callback fired.
	}
}

func TestWatch_StopWatchPreventsCallbacks(t *testing.T) {
	dir := t.TempDir()
	r, err := Load(dir)
	require.NoError(t, err)
	require.NoError(t, r.Add("work", "/tmp/work.db"))

	called := make(chan struct{}, 1)
	r.OnSwitch(func(name, path string) {
		called <- struct{}{}
	})

	ctx, cancel := context.WithCancel(context.Background())
	watchDone := make(chan struct{})
	go func() {
		_ = r.Watch(ctx)
		close(watchDone)
	}()

	time.Sleep(200 * time.Millisecond)

	// Stop the watcher.
	r.StopWatch()
	cancel()
	<-watchDone

	// Switch after stop — no callback should fire.
	r2, err := Load(dir)
	require.NoError(t, err)
	require.NoError(t, r2.Switch("work"))

	select {
	case <-called:
		t.Fatal("callback should not fire after StopWatch")
	case <-time.After(500 * time.Millisecond):
		// OK.
	}
}

func TestWatch_DebouncesRapidWrites(t *testing.T) {
	dir := t.TempDir()
	r, err := Load(dir)
	require.NoError(t, err)
	require.NoError(t, r.Add("work", "/tmp/work.db"))
	require.NoError(t, r.Add("personal", "/tmp/personal.db"))

	var callCount int32
	var lastName string
	done := make(chan struct{}, 10)
	r.OnSwitch(func(name, path string) {
		atomic.AddInt32(&callCount, 1)
		lastName = name
		done <- struct{}{}
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = r.Watch(ctx)
	}()

	time.Sleep(200 * time.Millisecond)

	// Rapid-fire switches: default → work → personal (within debounce window).
	r2, err := Load(dir)
	require.NoError(t, err)
	require.NoError(t, r2.Switch("work"))
	require.NoError(t, r2.Switch("personal"))

	select {
	case <-done:
		// Wait a bit more to see if extra callbacks arrive.
		time.Sleep(300 * time.Millisecond)
		count := atomic.LoadInt32(&callCount)
		assert.Equal(t, int32(1), count, "debounce should coalesce rapid writes into a single callback")
		assert.Equal(t, "personal", lastName, "should reflect the final state")
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for OnSwitch callback")
	}
}
```

Add these imports to the test file's import block: `"context"`, `"sync/atomic"`, `"time"`.

- [ ] **Step 2: Run tests to verify they fail**

Run:
```bash
go test ./internal/ledger/ -run "TestWatch" -v
```
Expected: compilation errors — `OnSwitch`, `Watch`, `StopWatch` not defined.

- [ ] **Step 3: Implement Registry watcher**

In `internal/ledger/registry.go`, add these imports: `"context"`, `"sync"`, `"time"`, `"github.com/fsnotify/fsnotify"`.

Add new fields to the `Registry` struct:

```go
type Registry struct {
	ActiveLedger   string           `yaml:"active"`
	Ledgers        map[string]Entry `yaml:"ledgers"`
	MigratedLegacy bool             `yaml:"-"`
	filePath       string
	mu             sync.Mutex
	callbacks      []func(name string, path string)
	watcher        *fsnotify.Watcher
}
```

Add these methods after the existing methods:

```go
func (r *Registry) OnSwitch(fn func(name, path string)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.callbacks = append(r.callbacks, fn)
}

func (r *Registry) Watch(ctx context.Context) error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("create watcher: %w", err)
	}
	r.mu.Lock()
	r.watcher = w
	r.mu.Unlock()

	defer w.Close()

	if err := w.Add(r.filePath); err != nil {
		return fmt.Errorf("watch %s: %w", r.filePath, err)
	}

	var debounce *time.Timer
	for {
		select {
		case <-ctx.Done():
			if debounce != nil {
				debounce.Stop()
			}
			return ctx.Err()
		case event, ok := <-w.Events:
			if !ok {
				return nil
			}
			if event.Op&(fsnotify.Write|fsnotify.Create) == 0 {
				continue
			}
			if debounce != nil {
				debounce.Stop()
			}
			debounce = time.AfterFunc(100*time.Millisecond, func() {
				r.reload()
			})
		case err, ok := <-w.Errors:
			if !ok {
				return nil
			}
			fmt.Fprintf(os.Stderr, "ledger watcher error: %v\n", err)
		}
	}
}

func (r *Registry) StopWatch() {
	r.mu.Lock()
	w := r.watcher
	r.watcher = nil
	r.mu.Unlock()
	if w != nil {
		_ = w.Close()
	}
}

func (r *Registry) reload() {
	data, err := os.ReadFile(r.filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "reload ledgers.yaml: %v\n", err)
		return
	}
	var fresh Registry
	if err := yaml.Unmarshal(data, &fresh); err != nil {
		fmt.Fprintf(os.Stderr, "parse ledgers.yaml: %v\n", err)
		return
	}

	prev := r.ActiveLedger
	if fresh.ActiveLedger == prev {
		return
	}

	r.ActiveLedger = fresh.ActiveLedger
	r.Ledgers = fresh.Ledgers
	if r.Ledgers == nil {
		r.Ledgers = make(map[string]Entry)
	}

	entry, exists := r.Ledgers[r.ActiveLedger]
	if !exists {
		fmt.Fprintf(os.Stderr, "reloaded active ledger %q not found in registry\n", r.ActiveLedger)
		return
	}

	r.mu.Lock()
	cbs := make([]func(string, string), len(r.callbacks))
	copy(cbs, r.callbacks)
	r.mu.Unlock()

	for _, fn := range cbs {
		fn(r.ActiveLedger, entry.Path)
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run:
```bash
go test ./internal/ledger/ -run "TestWatch" -v -count=1
```
Expected: all 4 `TestWatch_*` tests pass.

- [ ] **Step 5: Run full ledger test suite to check for regressions**

Run:
```bash
go test ./internal/ledger/ -v -count=1
```
Expected: all tests pass.

- [ ] **Step 6: Commit**

```bash
git add internal/ledger/registry.go internal/ledger/registry_test.go
git commit -m "feat(ledger): add file-watch with OnSwitch callbacks (#119)

Registry watches ledgers.yaml for changes and fires registered
callbacks when the active ledger changes. Includes 100ms debounce."
```

---

### Task 3: Store — Add RWMutex and read-lock to sqlite.go

**Files:**
- Modify: `internal/store/sqlite.go`
- Modify: `internal/store/export_test.go`

- [ ] **Step 1: Run existing store tests to confirm green baseline**

Run:
```bash
go test ./internal/store/ -v -count=1
```
Expected: all tests pass.

- [ ] **Step 2: Add `mu sync.RWMutex` to Store and read-lock `ExecTx`, `Close`, `DB`**

In `internal/store/sqlite.go`, add `"sync"` to imports.

Update `Store` struct:

```go
type Store struct {
	db    DBTX
	rawDB *sql.DB
	mu    sync.RWMutex
}
```

Update `ExecTx`:

```go
func (s *Store) ExecTx(ctx context.Context, fn func(repository.Repository) error) error {
	s.mu.RLock()
	db, ok := s.db.(*sql.DB)
	s.mu.RUnlock()
	if !ok {
		return fmt.Errorf("store is already in a transaction")
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	txStore := &Store{db: tx}

	err = fn(txStore)
	if err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("tx err: %v, rb err: %v", err, rbErr)
		}
		return err
	}
	return tx.Commit()
}
```

Update `Close`:

```go
func (s *Store) Close() error {
	s.mu.RLock()
	db, ok := s.db.(*sql.DB)
	s.mu.RUnlock()
	if ok {
		return db.Close()
	}
	return nil
}
```

Update `DB`:

```go
func (s *Store) DB() *sql.DB {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.rawDB
}
```

Update `export_test.go` — the `QueryRowContext` helper also accesses `s.db`:

```go
func (s *Store) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.db.QueryRowContext(ctx, query, args...)
}
```

- [ ] **Step 3: Run store tests to verify nothing broke**

Run:
```bash
go test ./internal/store/ -v -count=1
```
Expected: all tests pass.

- [ ] **Step 4: Commit**

```bash
git add internal/store/sqlite.go internal/store/export_test.go
git commit -m "refactor(store): add RWMutex to Store for safe DB swap (#119)

Read-lock ExecTx, Close, DB, and QueryRowContext (export_test).
Prepares for Swap method and per-method read-locks."
```

---

### Task 4: Store — Add read-lock to sqlite_account.go methods

**Files:**
- Modify: `internal/store/sqlite_account.go`

- [ ] **Step 1: Add read-lock to every method that accesses `s.db` or `s.rawDB`**

For each method in `sqlite_account.go`, add `s.mu.RLock()` and `defer s.mu.RUnlock()` as the first two lines of the method body. The methods are:

1. `CreateAccount` — first line of method body
2. `GetAllAccounts` — first line of method body
3. `GetAccountByName` — first line of method body
4. `GetAccountByID` — first line of method body
5. `AccountExists` — first line of method body
6. `GetAllAccountBalances` — first line of method body
7. `HasChildAccounts` — first line of method body
8. `GetAccountsByType` — first line of method body
9. `GetAccountBalance` — first line of method body
10. `scanAccounts` — first line of method body
11. `AccountHasTransactions` — first line of method body
12. `DeleteAccount` — first line of method body
13. `UpdateAccountMetadata` — first line of method body

For `RenameAccount`, which uses `s.rawDB` directly:

```go
func (s *Store) RenameAccount(ctx context.Context, oldName, newName string) error {
	s.mu.RLock()
	rawDB := s.rawDB
	s.mu.RUnlock()
	if rawDB == nil {
		return fmt.Errorf("store is already in a transaction")
	}

	tx, err := rawDB.BeginTx(ctx, nil)
	// ... rest unchanged ...
}
```

- [ ] **Step 2: Run account store tests**

Run:
```bash
go test ./internal/store/ -run "Account" -v -count=1
```
Expected: all account tests pass.

- [ ] **Step 3: Commit**

```bash
git add internal/store/sqlite_account.go
git commit -m "refactor(store): add read-lock to account query methods (#119)"
```

---

### Task 5: Store — Add read-lock to sqlite_transaction.go methods

**Files:**
- Modify: `internal/store/sqlite_transaction.go`

- [ ] **Step 1: Add read-lock to every method that accesses `s.db`**

For each method in `sqlite_transaction.go`, add `s.mu.RLock()` and `defer s.mu.RUnlock()` as the first two lines of the method body. The methods are:

1. `CreateTransactionWithSplits`
2. `GetTransactionByID`
3. `GetTransactionsByAccount`
4. `GetTransactionsByDateRange`
5. `GetAllTransactions`
6. `UpdateTransactionStatus`
7. `DeleteTransaction`
8. `UpdateTransactionBasic`
9. `UpdateSplit`
10. `DeleteSplit`
11. `CreateSplit`
12. `GetSplitsByTransaction`
13. `GetSplitsWithAccountsByDateRange`
14. `GetSplitsWithAccountsByTransaction`
15. `GetSplitsWithAccountsByTransactionIDs`
16. `ListTransactions`
17. `ListTransactionsByAccount`
18. `scanTransactions`

- [ ] **Step 2: Run transaction store tests**

Run:
```bash
go test ./internal/store/ -run "Transaction" -v -count=1
```
Expected: all transaction tests pass.

- [ ] **Step 3: Commit**

```bash
git add internal/store/sqlite_transaction.go
git commit -m "refactor(store): add read-lock to transaction query methods (#119)"
```

---

### Task 6: Store — Add read-lock to sqlite_reconcile.go methods

**Files:**
- Modify: `internal/store/sqlite_reconcile.go`

- [ ] **Step 1: Add read-lock to every method that accesses `s.db`**

For each method in `sqlite_reconcile.go`, add `s.mu.RLock()` and `defer s.mu.RUnlock()` as the first two lines of the method body. The methods are:

1. `GetUnreconciledTransactionsByAccount`
2. `MarkSplitsReconciledByAccount`
3. `BulkUpdateTransactionStatus`
4. `GetLastReconciledBalance`
5. `SetLastReconciledBalance`

- [ ] **Step 2: Run reconcile store tests**

Run:
```bash
go test ./internal/store/ -run "Reconcil" -v -count=1
```
Expected: all reconcile tests pass.

- [ ] **Step 3: Run full store test suite**

Run:
```bash
go test ./internal/store/ -v -count=1
```
Expected: all tests pass.

- [ ] **Step 4: Commit**

```bash
git add internal/store/sqlite_reconcile.go
git commit -m "refactor(store): add read-lock to reconcile query methods (#119)"
```

---

### Task 7: Store — Swap method

**Files:**
- Modify: `internal/store/sqlite.go`
- Create: `internal/store/sqlite_swap_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/store/sqlite_swap_test.go`:

```go
package store_test

import (
	"context"
	"path/filepath"
	"sync"
	"testing"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/repository"
	"github.com/hance08/kea/internal/store"
	"github.com/hance08/kea/migrations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSwap_QueriesHitNewDB(t *testing.T) {
	dir := t.TempDir()
	dbA := filepath.Join(dir, "a.db")
	dbB := filepath.Join(dir, "b.db")

	s, err := store.NewStore(dbA, migrations.FS)
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	ctx := context.Background()

	// Create an account in DB A.
	err = s.ExecTx(ctx, func(repo repository.Repository) error {
		_, err := repo.CreateAccount(ctx, "Assets:BankA", model.AccountTypeAsset, "USD", "", nil)
		return err
	})
	require.NoError(t, err)

	// Pre-create DB B with its own account.
	sB, err := store.NewStore(dbB, migrations.FS)
	require.NoError(t, err)
	err = sB.ExecTx(ctx, func(repo repository.Repository) error {
		_, err := repo.CreateAccount(ctx, "Assets:BankB", model.AccountTypeAsset, "USD", "", nil)
		return err
	})
	require.NoError(t, err)
	require.NoError(t, sB.Close())

	// Swap to DB B.
	err = s.Swap(dbB, migrations.FS)
	require.NoError(t, err)

	// After swap, BankA should not exist; BankB should.
	_, err = s.GetAccountByName(ctx, "Assets:BankA")
	assert.Error(t, err, "BankA should not exist in DB B")

	acc, err := s.GetAccountByName(ctx, "Assets:BankB")
	require.NoError(t, err)
	assert.Equal(t, "Assets:BankB", acc.Name)
}

func TestSwap_FailedSwapKeepsOldConnection(t *testing.T) {
	dir := t.TempDir()
	dbA := filepath.Join(dir, "a.db")

	s, err := store.NewStore(dbA, migrations.FS)
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	ctx := context.Background()

	err = s.ExecTx(ctx, func(repo repository.Repository) error {
		_, err := repo.CreateAccount(ctx, "Assets:BankA", model.AccountTypeAsset, "USD", "", nil)
		return err
	})
	require.NoError(t, err)

	// Swap to an invalid path — should fail.
	err = s.Swap("/nonexistent/dir/bad.db", migrations.FS)
	require.Error(t, err)

	// Old connection should still work.
	acc, err := s.GetAccountByName(ctx, "Assets:BankA")
	require.NoError(t, err)
	assert.Equal(t, "Assets:BankA", acc.Name)
}

func TestSwap_ConcurrentReads(t *testing.T) {
	dir := t.TempDir()
	dbA := filepath.Join(dir, "a.db")
	dbB := filepath.Join(dir, "b.db")

	s, err := store.NewStore(dbA, migrations.FS)
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	ctx := context.Background()

	err = s.ExecTx(ctx, func(repo repository.Repository) error {
		_, err := repo.CreateAccount(ctx, "Assets:BankA", model.AccountTypeAsset, "USD", "", nil)
		return err
	})
	require.NoError(t, err)

	// Pre-create DB B.
	sB, err := store.NewStore(dbB, migrations.FS)
	require.NoError(t, err)
	err = sB.ExecTx(ctx, func(repo repository.Repository) error {
		_, err := repo.CreateAccount(ctx, "Assets:BankB", model.AccountTypeAsset, "USD", "", nil)
		return err
	})
	require.NoError(t, err)
	require.NoError(t, sB.Close())

	// Launch concurrent readers and a swap in the middle.
	var wg sync.WaitGroup
	errs := make(chan error, 20)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.GetAllAccounts(ctx)
			if err != nil {
				errs <- err
			}
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := s.Swap(dbB, migrations.FS); err != nil {
			errs <- err
		}
	}()

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("concurrent operation failed: %v", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:
```bash
go test ./internal/store/ -run "TestSwap" -v -count=1
```
Expected: compilation error — `Swap` method not defined.

- [ ] **Step 3: Implement the Swap method**

Add this method to `internal/store/sqlite.go`, after `Close()`:

```go
func (s *Store) Swap(newPath string, migrationsFS fs.FS) error {
	newStore, err := NewStore(newPath, migrationsFS)
	if err != nil {
		return fmt.Errorf("open new database: %w", err)
	}

	s.mu.Lock()
	oldDB := s.rawDB
	s.rawDB = newStore.rawDB
	s.db = newStore.db
	s.mu.Unlock()

	if oldDB != nil {
		_ = oldDB.Close()
	}

	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run:
```bash
go test ./internal/store/ -run "TestSwap" -v -count=1
```
Expected: all 3 `TestSwap_*` tests pass.

- [ ] **Step 5: Run full store test suite**

Run:
```bash
go test ./internal/store/ -v -count=1
```
Expected: all tests pass.

- [ ] **Step 6: Commit**

```bash
git add internal/store/sqlite.go internal/store/sqlite_swap_test.go
git commit -m "feat(store): add Swap method for graceful DB reconnection (#119)

Opens new connection first, swaps under write lock, then closes old
connection. Failed swap leaves old connection intact."
```

---

### Task 8: App wiring — NewApp signature + callback

**Files:**
- Modify: `internal/app/app.go`
- Modify: `cmd/root.go`

- [ ] **Step 1: Update App struct and NewApp signature**

In `internal/app/app.go`, add `"github.com/hance08/kea/internal/ledger"` to imports.

Update `App` struct:

```go
type App struct {
	Service  *service.Service
	Registry *ledger.Registry
}
```

Update `NewApp` signature and body:

```go
func NewApp(cfg *config.Config, registry *ledger.Registry, migrationFS fs.FS) (*App, func(), error) {
	dbPathRaw := cfg.Database.Path

	if dbPathRaw == "" {
		appDir, err := GetAppDataDir()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to determine data directory: %w", err)
		}
		dbPathRaw = filepath.Join(appDir, "kea.db")
	}

	if err := backup.Run(dbPathRaw, nil); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: backup failed: %v\n", err)
	}

	dbStore, err := store.NewStore(dbPathRaw, migrationFS)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	svc := service.NewService(dbStore, dbStore, dbStore, cfg)

	registry.OnSwitch(func(name, path string) {
		if err := dbStore.Swap(path, migrationFS); err != nil {
			fmt.Fprintf(os.Stderr, "ledger switch failed: %v\n", err)
			return
		}
		cfg.ActiveLedger = name
	})

	cleanup := func() {
		registry.StopWatch()
		if err := dbStore.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Error closing DB: %v\n", err)
		}
	}

	return &App{
		Service:  svc,
		Registry: registry,
	}, cleanup, nil
}
```

- [ ] **Step 2: Update cmd/root.go to pass registry**

In `cmd/root.go`, change line 115 from:

```go
application, cleanup, err := app.NewApp(cfg, migrations)
```

to:

```go
application, cleanup, err := app.NewApp(cfg, registry, migrations)
```

- [ ] **Step 3: Verify compilation**

Run:
```bash
go build ./...
```
Expected: success.

- [ ] **Step 4: Run full test suite**

Run:
```bash
go test ./... -count=1
```
Expected: all tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/app/app.go cmd/root.go
git commit -m "feat(app): wire Registry OnSwitch to Store Swap (#119)

NewApp registers an OnSwitch callback that swaps the Store DB and
updates cfg.ActiveLedger. Cleanup calls StopWatch before Close."
```

---

### Task 9: End-to-end verification

**Files:** None (verification only)

- [ ] **Step 1: Run the full test suite**

Run:
```bash
go test ./... -v -count=1
```
Expected: all tests pass across all packages.

- [ ] **Step 2: Build the binary**

Run:
```bash
make build
```
Expected: builds successfully.

- [ ] **Step 3: Smoke test CLI commands**

Run:
```bash
./kea_test ledger list
./kea_test info
```
Expected: both commands work as before — no behavior change for CLI.
