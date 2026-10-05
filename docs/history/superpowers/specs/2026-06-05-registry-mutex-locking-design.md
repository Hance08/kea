# Fix: `ledger.Registry` mutex locking discipline

**Status:** Design approved 2026-06-05.
**Trigger:** Pre-existing concurrency bug materialized by the web API workstream (PRs #175–#180). Single-threaded CLI usage happened to mask it; multi-goroutine API request handlers do not.

## Problem

`internal/ledger/registry.go` declares `sync.Mutex` on `Registry` but only `OnSwitch`, `Watch`, `StopWatch`, and `reload` hold it. Every other method that touches `r.ActiveLedger`, the `r.Ledgers` map, or both is unsynchronized:

| Field | Synchronized writers | Unsynchronized accessors |
|---|---|---|
| `r.ActiveLedger` (string) | `reload` (W) | `Switch` (W), `ActiveName` (R), `Remove` (R via `ActiveName`), `Save` via yaml marshal (R) |
| `r.Ledgers` (map[string]Entry) | `reload` (W replace) | `Add` (R/W), `Switch` (R), `Remove` (R/W), `Names` (R iter), `EntryFor` (R), `Active` (R), `Save` via yaml marshal (R) |
| `r.callbacks` | `OnSwitch` (W), `reload` (R copy) | — already correct |
| `r.watcher` | `Watch` (R/W), `StopWatch` (R/W) | — already correct |

`r.MigratedLegacy` and `r.filePath` are written only by `Load` (single-threaded init) and never mutated afterward; unlocked reads remain safe.

The web API materializes the race in two ways:
- HTTP handlers call `Names`, `EntryFor`, `Active`, `ActiveName` concurrently from goroutines.
- The fsnotify watcher's `reload()` rewrites `r.Ledgers` and `r.ActiveLedger` on debounced file events.

The actual failure mode is **the Go runtime's panic on concurrent map read/write** — not just a `-race` detector warning. The CLI happened to be safe because all method calls came from `main()`'s single goroutine; the watcher goroutine was the only second actor, and the short-circuit `if fresh.ActiveLedger == prev` in `reload()` made racing windows rare. The API server breaks both assumptions.

## Decision

Synchronize every accessor that touches `r.ActiveLedger`, `r.Ledgers`, or both, under the existing `sync.Mutex`. Refactor `Save` so methods that already hold the lock can persist without re-entering it. Add a concurrent-stress test and a `Makefile` target so `-race` regressions surface in normal development.

Three options were considered:

1. **`sync.Mutex` + `saveLocked` helper, lock every accessor, add race tests** *(chosen)*. Standard Go pattern. Smallest diff that closes the surface.
2. `sync.RWMutex`. Read methods take `RLock`. Rejected: single-user localhost API has near-zero read contention; the extra API surface (and the class of bugs it enables — forgetting `RUnlock`, RLock-to-Lock upgrade attempts) buys nothing measurable.
3. Copy-on-write swap via `atomic.Pointer[innerState]`. Elegant for readers but turns every write into a full map clone and complicates `Save` (must atomically swap pointer with file write). Overkill for a four-field, low-mutation registry.

## Design

### Locking discipline

- `r.mu` (existing `sync.Mutex`) guards `r.ActiveLedger`, `r.Ledgers`, `r.callbacks`, and `r.watcher`.
- Every read accessor that touches those fields takes the lock for the whole body:
  - `Names` — iterate `r.Ledgers` under the lock, build the sorted slice, release.
  - `EntryFor` — read the map under the lock, return the entry value.
  - `Active` — read `r.ActiveLedger` (via inlined `ActiveName` body, see below) and look up `r.Ledgers` under the lock.
  - `ActiveName` — read `r.ActiveLedger` under the lock. `KEA_LEDGER` env lookup stays outside the lock.
- Every mutating method takes the lock and calls the new private `saveLocked()` at the end before releasing:
  - `Add` — duplicate-check + insert + persist, all under the lock.
  - `Switch` — existence check + write `r.ActiveLedger` + persist, all under the lock.
  - `Remove` — active-check + lookup + delete + persist under the lock. The post-persist `os.Remove(entry.Path)` for `deleteFile=true` happens **outside** the lock because it's disk I/O on the deleted entry's path (already captured locally), and holding `r.mu` across arbitrary filesystem latency would block all registry reads.
- `Save()` (exported, currently called only from inside the package) takes the lock then calls `saveLocked()`. Kept exported because removing it would be an unnecessary API break.
- `reload()` already takes the lock around the in-memory update; unchanged. Callbacks are invoked **outside** the lock (current behavior, preserved) because the `app.OnSwitch` callback calls `store.Swap`, which touches disk and acquires its own mutex; holding `r.mu` across that would invert lock ordering and serialize all registry reads behind a ledger switch.

### `saveLocked` refactor

```go
// saveLocked writes the current registry state to ledgers.yaml.
// Callers must hold r.mu.
func (r *Registry) saveLocked() error {
    // body of the current Save(): mkdir, marshal, write.
}

// Save persists the registry to disk under r.mu.
func (r *Registry) Save() error {
    r.mu.Lock()
    defer r.mu.Unlock()
    return r.saveLocked()
}
```

Internal callers — `Add`, `Switch`, `Remove`, and the bootstrap path in `Load` — call `saveLocked()` directly while holding the lock. `Load` is single-threaded init; it can either take and release the lock around `saveLocked()` for symmetry, or skip the lock since no other goroutine exists yet. **Chosen:** call `saveLocked()` without the lock from `Load` (one-shot init path, taking the lock would just be ceremony).

### `ActiveName` and the `KEA_LEDGER` env

Current code:

```go
func (r *Registry) ActiveName() string {
    if env := os.Getenv("KEA_LEDGER"); env != "" {
        return env
    }
    return r.ActiveLedger
}
```

`os.Getenv` is thread-safe and independent of registry state. The new shape:

```go
func (r *Registry) ActiveName() string {
    if env := os.Getenv("KEA_LEDGER"); env != "" {
        return env
    }
    r.mu.Lock()
    defer r.mu.Unlock()
    return r.ActiveLedger
}
```

`Remove` currently calls `r.ActiveName()` to check whether the target is active. With `ActiveName` now taking the lock, `Remove` cannot call it while holding the lock without deadlocking. Two clean options:

- **(a)** Inline the active-name read in `Remove`: read `r.ActiveLedger` (and the env) directly under the lock.
- **(b)** Add a private `activeNameLocked()` helper that `Remove` calls while holding the lock; `ActiveName` becomes a wrapper that takes the lock and calls it.

**Chosen: (b)** — the helper pattern composes more cleanly if other methods later need the same logic. Body:

```go
func (r *Registry) activeNameLocked() string {
    if env := os.Getenv("KEA_LEDGER"); env != "" {
        return env
    }
    return r.ActiveLedger
}
```

`Active` similarly inlines via `activeNameLocked()` while holding the lock.

### Race tests

Two new tests in `internal/ledger/registry_test.go`:

1. **`TestRegistry_ConcurrentAccess`** — spawn N goroutines (say N=8) that each loop for a fixed duration (say 100ms) issuing a randomized mix of `Add` (with a unique per-goroutine name suffix), `Switch` (to a known name), `EntryFor`, `Names`, `Active`, `ActiveName`. The test passes if no goroutine panics and no `-race` error fires. Designed to fail loudly (concurrent map panic) without `-race`, and to surface subtler data races with `-race`.

2. **`TestRegistry_SwitchVsReload`** — the exact production scenario. Start a `Watch` goroutine. In a second goroutine, call `Switch` between two existing names rapidly for ~200ms. Assert no panic, no `-race` failure. The watcher's `reload()` and the foreground `Switch` writes must coexist.

Both tests use `t.TempDir()` and `Load()` like the existing watch tests. Both are short enough (<1s total) to run by default in `go test ./internal/ledger/...`.

### Makefile target

Add a `test-race` target:

```makefile
test-race:
	go test -race ./...
```

Conforms to the existing `make build` / `make run` / `make test` style (per `CLAUDE.md`). Makes `-race` regressions cheap to spot during development without committing CI changes.

### Callbacks released outside the lock — preserve, document

`reload()` already copies the `r.callbacks` slice while holding the lock, then invokes them after releasing. The new `Switch`/`Add`/`Remove` paths **do not** trigger callbacks (only `reload` does, on detected external changes), so they don't need this dance. Worth a one-line comment in `reload()` reaffirming why the copy-and-release pattern is load-bearing.

## Out of scope

- **Save-failure rollback.** `Add`/`Switch`/`Remove` mutate in-memory state before calling `Save`/`saveLocked`. If the disk write fails, the in-memory state diverges from the persisted file. This is pre-existing behavior; fixing it is a separate ergonomics call (probably "revert in-memory state on persist failure") that's orthogonal to the concurrency surface.
- **`OnSwitch` callbacks mutating `*config.Config` from `app.go:50`.** Real adjacent race against API readers of `cfg.ActiveLedger` / `cfg.Database.Path`; the fix shape is different (probably move runtime-mutable state off `*config.Config` to `*App`) and deserves its own brainstorm. Spawned as a separate follow-up task.
- **`cmd/ledger/*.go` runner structs.** CLI-only, single-threaded.
- **Lock-free read fast paths or sharding.** No demonstrated contention to justify the complexity.

## Verification

- `go test ./...` — green.
- `go test -race ./internal/ledger/...` — green; the two new race tests exercise the surface.
- `go test -race ./...` — green; catches incidental races elsewhere (none expected from this change).
- `go build ./...` — clean.
- `make test-race` — equivalent to the second command; just a workflow shortcut.

## Commit shape

Single commit, scope `(ledger)`:

```
fix(ledger): synchronize all Registry accessors under r.mu

The Registry's sync.Mutex was held only by OnSwitch/Watch/StopWatch/reload.
Add/Switch/Remove and the read accessors (Names/EntryFor/Active/ActiveName)
touched r.ActiveLedger and r.Ledgers without the lock. The CLI happened to be
safe because all calls came from a single goroutine, but the web API server
exposes the race: concurrent request handlers calling EntryFor while the
fsnotify watcher's reload() rewrites r.Ledgers triggers Go's concurrent-map
panic, not just a -race detector warning.

This commit:
- Locks every accessor that reads or writes the guarded fields.
- Extracts saveLocked() so mutators can persist without re-entering r.mu.
- Adds activeNameLocked() so Remove can read the active name while holding
  the lock.
- Preserves the existing "copy callbacks under the lock, invoke after
  release" pattern in reload().
- Adds TestRegistry_ConcurrentAccess and TestRegistry_SwitchVsReload to
  exercise the surface under -race.
- Adds `make test-race` so -race becomes a one-liner during development.
```

The Makefile change is small enough to ride along with the code change rather than being its own commit.
