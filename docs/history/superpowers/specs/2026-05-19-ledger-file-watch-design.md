# Ledger File-Watch Design

**Issue:** #119 — CLI ledger switch is invisible to running web server (split-brain)

**Date:** 2026-05-19

## Problem

The ledger registry (`ledgers.yaml`) is loaded once at startup and held in memory.
If a long-lived process (web server) is running with ledger A and the CLI switches to
ledger B, the server's in-memory registry and Store still point to ledger A. The user
sees inconsistent data between CLI and web — a silent split-brain.

## Solution

Registry-owned file-watch with graceful Store swap. The Registry watches `ledgers.yaml`
for changes and fires callbacks when the active ledger changes. The Store swaps its DB
connection gracefully using a read-write mutex so in-flight queries complete safely.

## Design

### 1. Registry — File Watcher & Callbacks

**File:** `internal/ledger/registry.go`

New fields on `Registry`:

- `mu sync.Mutex` — protects the callbacks slice
- `callbacks []func(name string, path string)` — switch listeners
- `watcher *fsnotify.Watcher` — nil until `Watch()` is called

New methods:

- `OnSwitch(fn func(name, path string))` — registers a callback fired when the active
  ledger changes due to a file-watch reload.
- `Watch(ctx context.Context) error` — starts an fsnotify watcher on `ledgers.yaml`.
  On file write events, re-reads the YAML. If `ActiveLedger` changed compared to the
  previous in-memory value, resolves the new DB path and fires all registered callbacks.
  Blocks until ctx is cancelled. Uses a 100ms debounce timer to coalesce rapid
  file-write events.
- `StopWatch()` — closes the fsnotify watcher. Idempotent.

Thread safety: the callbacks slice is protected by `mu`. Registry fields (`ActiveLedger`,
`Ledgers`) are only mutated by the watcher goroutine during reload — no concurrent
writes since the CLI process that triggered the switch is a separate OS process.

**New dependency:** `github.com/fsnotify/fsnotify`

### 2. Store — Graceful DB Swap

**File:** `internal/store/sqlite.go`

New field on `Store`:

- `mu sync.RWMutex` — protects `db` and `rawDB` during swap

New method:

- `Swap(newPath string, migrationsFS fs.FS) error` — opens a new SQLite connection to
  `newPath`, runs migrations, acquires write lock, swaps `rawDB` and `db`, releases
  lock, closes old connection. If the new connection fails, the old connection remains
  active and the error is returned.

Query method changes: all Store methods that access `s.db` acquire `s.mu.RLock()` at
entry and defer `s.mu.RUnlock()`. This includes:

- All account query methods (sqlite_account.go)
- All transaction query methods (sqlite_transaction.go)
- `ExecTx` — acquires read lock to get `*sql.DB` and begin the transaction, then
  releases it. The child `txStore` has no mutex (scoped to one goroutine, no swap).
- `Close` and `DB` methods

### 3. App Wiring & Lifecycle

**File:** `internal/app/app.go`

`App` struct change:

- Add `Registry *ledger.Registry` field

`NewApp` signature change:

```go
func NewApp(cfg *config.Config, registry *ledger.Registry, migrationFS fs.FS) (*App, func(), error)
```

Callback wiring (inside `NewApp`, after Store and Service creation):

```go
registry.OnSwitch(func(name, path string) {
    if err := dbStore.Swap(path, migrationFS); err != nil {
        fmt.Fprintf(os.Stderr, "ledger switch failed: %v\n", err)
        return
    }
    cfg.ActiveLedger = name
})
```

Cleanup function calls `registry.StopWatch()` before closing the Store.

**Watch is started by the caller, not NewApp.** Only long-lived processes (future
`serve` command) call `registry.Watch(ctx)` in a goroutine. Current CLI commands do
not start the watcher — zero behavior change for existing commands.

**File:** `cmd/root.go`

Pass registry to `NewApp`:

```go
application, cleanup, err := app.NewApp(cfg, registry, migrations)
```

### 4. Testing Strategy

**Registry watcher tests** (`internal/ledger/registry_test.go`):

- Test that modifying `ledgers.yaml` on disk triggers the `OnSwitch` callback with the
  correct name and path.
- Test debouncing: rapid writes produce a single callback.
- Test that `StopWatch()` stops the watcher and no further callbacks fire.
- Test that unchanged `ActiveLedger` after a file write does not fire callbacks.

**Store swap tests** (`internal/store/sqlite_swap_test.go`):

- Test that `Swap` to a new DB path results in queries hitting the new database.
- Test that a failed `Swap` (invalid path) leaves the old connection intact.
- Test concurrent reads during a swap complete without error.

**App integration** — covered by verifying the callback wiring connects Registry
change to Store swap. No new app-level tests needed beyond the unit tests above.

### 5. Files Changed

| File | Change |
|------|--------|
| `go.mod` / `go.sum` | Add `github.com/fsnotify/fsnotify` |
| `internal/ledger/registry.go` | Add `mu`, `callbacks`, `watcher` fields; `OnSwitch`, `Watch`, `StopWatch` methods; internal `reload` helper |
| `internal/ledger/registry_test.go` | New: watcher and callback tests |
| `internal/store/sqlite.go` | Add `mu` field; `Swap` method; read-lock in `ExecTx`, `Close`, `DB` |
| `internal/store/sqlite_account.go` | Add read-lock to each method |
| `internal/store/sqlite_transaction.go` | Add read-lock to each method |
| `internal/store/sqlite_swap_test.go` | New: swap and concurrency tests |
| `internal/app/app.go` | Add `Registry` to `App`; update `NewApp` signature; wire callback; update cleanup |
| `cmd/root.go` | Pass `registry` to `NewApp` |

### 6. What This Does NOT Change

- CLI command behavior — short-lived commands never start the watcher.
- The `Switch()` method itself — it still writes `ledgers.yaml` to disk as before.
- The `KEA_LEDGER` env var override — still respected by `ActiveName()`. When set,
  the watcher still fires callbacks for `ActiveLedger` changes in the YAML, but
  `ActiveName()` continues to return the env var value. The callback updates
  `cfg.ActiveLedger` to mirror the registry's own field, not the resolved name.
