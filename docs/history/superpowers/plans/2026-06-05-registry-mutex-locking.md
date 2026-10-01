# `ledger.Registry` Mutex Locking Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close every unsynchronized accessor of `ledger.Registry`'s shared state (`r.ActiveLedger`, `r.Ledgers`) under the existing `r.mu`, so concurrent web-API request handlers and the fsnotify watcher's `reload()` cannot trigger Go's concurrent-map-access panic.

**Architecture:** Single-file refactor of `internal/ledger/registry.go`. Two private helpers — `saveLocked()` and `activeNameLocked()` — let mutating methods compose without lock re-entrance. Every exported method that touches the guarded fields takes `r.mu` once and holds it for the full body, except `Remove`'s post-delete file I/O which runs outside the lock. Two new stress tests in `registry_test.go` exercise the surface and `make test-race` invokes them under `-race`.

**Tech Stack:** Go stdlib (`sync`, `os`, `errors`, `fmt`, `context`, `time`), `gopkg.in/yaml.v3`, `github.com/fsnotify/fsnotify`, `testify/{assert,require}`.

**Spec:** [`docs/superpowers/specs/2026-06-05-registry-mutex-locking-design.md`](../specs/2026-06-05-registry-mutex-locking-design.md)

---

## File Map

- Modify: `internal/ledger/registry.go` — refactor `Save`, `ActiveName`; lock `Names`, `EntryFor`, `Active`, `ActiveName`, `Add`, `Switch`, `Remove`. Introduce private `saveLocked`, `activeNameLocked`, and an unexported `removeAndSave` helper to isolate `Remove`'s locked region from its disk-I/O tail.
- Modify: `internal/ledger/registry_test.go` — append two new top-level tests: `TestRegistry_ConcurrentAccess` and `TestRegistry_SwitchVsReload`.
- Modify: `Makefile` — append a `test-race` target.

No new files. No new imports needed for production code (`sync` already imported). The test file already imports `sync`, `sync/atomic`, `time`, `context`, `os`, `path/filepath`, `testing`, `testify/assert`, `testify/require` — the new tests also need `fmt`, which is **not** currently in the import block. Add it.

---

### Task 1: Add the two failing race tests

**Files:**
- Modify: `internal/ledger/registry_test.go` — append two new test functions and add `"fmt"` to the import block.

The TDD red phase: with the current code, `TestRegistry_ConcurrentAccess` is overwhelmingly likely to crash with "fatal error: concurrent map read and map write" (Go runtime panic, not a recoverable test failure). `TestRegistry_SwitchVsReload` will be detected as a data race under `-race`. Both confirm the bug exists before we fix it.

- [ ] **Step 1: Add `"fmt"` to the import block in `internal/ledger/registry_test.go`**

The current import block (lines 6–18) is:

```go
import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)
```

Add `"fmt"` in alphabetical order between `"errors"` and `"os"`:

```go
import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)
```

- [ ] **Step 2: Append `TestRegistry_ConcurrentAccess` to the end of `registry_test.go`**

```go
// TestRegistry_ConcurrentAccess stresses every Registry accessor that touches
// r.ActiveLedger or r.Ledgers from multiple goroutines. With unsynchronized
// access (the pre-fix state) Go's runtime panics with "concurrent map read
// and map write" — that's the loud failure mode this test pins. Run under
// -race for additional data-race coverage.
func TestRegistry_ConcurrentAccess(t *testing.T) {
	dir := t.TempDir()
	r, err := Load(dir)
	require.NoError(t, err)
	require.NoError(t, r.Add("seed1", "/tmp/seed1.db"))
	require.NoError(t, r.Add("seed2", "/tmp/seed2.db"))

	const goroutines = 8
	const dur = 100 * time.Millisecond

	var wg sync.WaitGroup
	stop := make(chan struct{})
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			i := 0
			for {
				select {
				case <-stop:
					return
				default:
				}
				switch i % 5 {
				case 0:
					name := fmt.Sprintf("g%d-%d", id, i)
					_ = r.Add(name, "/tmp/"+name+".db")
				case 1:
					_ = r.Switch("seed1")
				case 2:
					_, _ = r.EntryFor("seed1")
				case 3:
					_ = r.Names()
				case 4:
					_ = r.ActiveName()
				}
				i++
			}
		}(g)
	}
	time.Sleep(dur)
	close(stop)
	wg.Wait()
}
```

- [ ] **Step 3: Append `TestRegistry_SwitchVsReload` immediately after**

```go
// TestRegistry_SwitchVsReload reproduces the production-shape race: the
// fsnotify watcher's reload() rewrites r.ActiveLedger and r.Ledgers under
// the lock, while an in-process Switch caller mutates them concurrently
// without it. Each Switch persists ledgers.yaml, which triggers reload()
// after the debounce window. With the fix in place this completes cleanly
// under -race; without the fix `-race` flags the data race on r.ActiveLedger.
func TestRegistry_SwitchVsReload(t *testing.T) {
	dir := t.TempDir()
	r, err := Load(dir)
	require.NoError(t, err)
	require.NoError(t, r.Add("work", "/tmp/work.db"))
	require.NoError(t, r.Add("personal", "/tmp/personal.db"))

	ctx, cancel := context.WithCancel(context.Background())
	watchDone := make(chan struct{})
	go func() {
		_ = r.Watch(ctx)
		close(watchDone)
	}()
	// Let the watcher install its fsnotify hook before we start mutating.
	time.Sleep(200 * time.Millisecond)

	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		if err := r.Switch("work"); err != nil {
			t.Fatalf("Switch(work): %v", err)
		}
		if err := r.Switch("personal"); err != nil {
			t.Fatalf("Switch(personal): %v", err)
		}
	}

	cancel()
	<-watchDone
}
```

- [ ] **Step 4: Run both new tests under `-race` and confirm they fail**

Run: `go test -race ./internal/ledger/ -run 'TestRegistry_ConcurrentAccess|TestRegistry_SwitchVsReload' -v`

Expected: at least one of the following two failure modes:

1. **`TestRegistry_ConcurrentAccess`**: a `fatal error: concurrent map read and map write` panic that aborts the test process (exit code non-zero), OR a `DATA RACE` report from the race detector on `r.Ledgers` accesses.
2. **`TestRegistry_SwitchVsReload`**: a `DATA RACE` report on `r.ActiveLedger` between `Switch` (the foreground writer) and `reload()` (the goroutine that holds the lock — the race is on `Switch`'s unsynchronized write versus `reload`'s locked read).

If both tests pass, the bug surface is not as described — STOP and re-examine. Do not proceed with the fix.

If the first test triggers a `fatal error: concurrent map ...` panic that aborts before the second test runs, that's also acceptable evidence — the bug is real, proceed to Task 2.

---

### Task 2: Introduce `saveLocked`; have `Save` delegate

**Files:**
- Modify: `internal/ledger/registry.go` — split the current `Save` method (lines 96–105) into `Save` (lock-taking wrapper) + `saveLocked` (lock-required body).

- [ ] **Step 1: Replace the current `Save` method**

Current code (lines 95–105):

```go
// Save writes the current registry state to ledgers.yaml.
func (r *Registry) Save() error {
	if err := os.MkdirAll(filepath.Dir(r.filePath), 0755); err != nil {
		return fmt.Errorf("create registry directory: %w", err)
	}
	data, err := yaml.Marshal(r)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	return os.WriteFile(r.filePath, data, 0644)
}
```

Replace with:

```go
// Save writes the current registry state to ledgers.yaml. Safe to call from
// external goroutines; takes r.mu internally.
func (r *Registry) Save() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.saveLocked()
}

// saveLocked persists the registry to disk. The caller MUST hold r.mu so
// that yaml.Marshal sees a consistent view of r.ActiveLedger and r.Ledgers.
func (r *Registry) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(r.filePath), 0755); err != nil {
		return fmt.Errorf("create registry directory: %w", err)
	}
	data, err := yaml.Marshal(r)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	return os.WriteFile(r.filePath, data, 0644)
}
```

- [ ] **Step 2: Update `Load` to call `saveLocked()` for its bootstrap save**

Find this block in `Load` (around line 89):

```go
	if err := r.Save(); err != nil {
		return nil, fmt.Errorf("init default ledger: %w", err)
	}
```

Replace with:

```go
	// Load runs during single-threaded init before any other goroutine
	// touches r; calling saveLocked() directly is safe and avoids the
	// uncontended lock cycle.
	if err := r.saveLocked(); err != nil {
		return nil, fmt.Errorf("init default ledger: %w", err)
	}
```

- [ ] **Step 3: Build and run the existing registry tests**

Run: `go test ./internal/ledger/ -v -run TestLoad`

Expected: all `TestLoad_*` tests PASS. The refactor is behavior-preserving for the single-threaded `Load` path; the new race tests still fail (we haven't applied any locking to the mutators yet).

If any pre-existing `TestLoad_*` test fails, STOP — the `Save`/`saveLocked` split has broken something. Inspect and fix before continuing.

---

### Task 3: Introduce `activeNameLocked`; have `ActiveName` delegate

**Files:**
- Modify: `internal/ledger/registry.go` — split the current `ActiveName` method (lines 155–162) into `ActiveName` (lock-taking wrapper) + `activeNameLocked` (lock-required body).

- [ ] **Step 1: Replace the current `ActiveName` method**

Current code:

```go
// ActiveName returns the name of the active ledger.
// KEA_LEDGER env var takes precedence over the registry's active field.
func (r *Registry) ActiveName() string {
	if env := os.Getenv("KEA_LEDGER"); env != "" {
		return env
	}
	return r.ActiveLedger
}
```

Replace with:

```go
// ActiveName returns the name of the active ledger. KEA_LEDGER env var
// takes precedence over the registry's active field. Safe to call from
// external goroutines; takes r.mu internally.
func (r *Registry) ActiveName() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.activeNameLocked()
}

// activeNameLocked returns the active ledger name. Caller MUST hold r.mu.
// The env-var lookup itself is thread-safe and independent of registry
// state but is held inside the lock to keep the function shape simple.
func (r *Registry) activeNameLocked() string {
	if env := os.Getenv("KEA_LEDGER"); env != "" {
		return env
	}
	return r.ActiveLedger
}
```

- [ ] **Step 2: Run the existing tests to confirm nothing regressed**

Run: `go test ./internal/ledger/ -v -run TestActive`

Expected: all `TestActive*` tests PASS (these are `TestActive_ReturnsResolvedPath`, `TestActive_NoActiveLedgerError`, etc.).

---

### Task 4: Lock the mutating methods (`Add`, `Switch`, `Remove`)

**Files:**
- Modify: `internal/ledger/registry.go` — apply locking to `Add` (lines 123–130), `Switch` (lines 132–139), and `Remove` (lines 167–185). Introduce a private `removeAndSave` helper so `Remove`'s post-delete file I/O can run outside the lock.

- [ ] **Step 1: Replace `Add`**

Current:

```go
// Add registers a new ledger. Returns ErrLedgerExists if the name is already taken.
func (r *Registry) Add(name, dbPath string) error {
	if _, exists := r.Ledgers[name]; exists {
		return fmt.Errorf("%w: %q", ErrLedgerExists, name)
	}
	r.Ledgers[name] = Entry{Path: dbPath}
	return r.Save()
}
```

Replace with:

```go
// Add registers a new ledger. Returns ErrLedgerExists if the name is already taken.
func (r *Registry) Add(name, dbPath string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.Ledgers[name]; exists {
		return fmt.Errorf("%w: %q", ErrLedgerExists, name)
	}
	r.Ledgers[name] = Entry{Path: dbPath}
	return r.saveLocked()
}
```

- [ ] **Step 2: Replace `Switch`**

Current:

```go
// Switch sets the active ledger by name. Returns ErrLedgerNotFound if unknown.
func (r *Registry) Switch(name string) error {
	if _, exists := r.Ledgers[name]; !exists {
		return fmt.Errorf("%w: %q — run: kea ledger list", ErrLedgerNotFound, name)
	}
	r.ActiveLedger = name
	return r.Save()
}
```

Replace with:

```go
// Switch sets the active ledger by name. Returns ErrLedgerNotFound if unknown.
func (r *Registry) Switch(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.Ledgers[name]; !exists {
		return fmt.Errorf("%w: %q — run: kea ledger list", ErrLedgerNotFound, name)
	}
	r.ActiveLedger = name
	return r.saveLocked()
}
```

- [ ] **Step 3: Replace `Remove` with the helper-based split**

Current:

```go
// Remove unregisters a ledger. If deleteFile is true and the ledger's DB file
// exists, it is deleted after unregistering. Returns ErrRemoveActive if the
// target is currently the active ledger.
func (r *Registry) Remove(name string, deleteFile bool) error {
	if name == r.ActiveName() {
		return ErrRemoveActive
	}
	entry, exists := r.Ledgers[name]
	if !exists {
		return fmt.Errorf("%w: %q", ErrLedgerNotFound, name)
	}
	delete(r.Ledgers, name)
	if err := r.Save(); err != nil {
		return err
	}
	if deleteFile {
		if err := os.Remove(entry.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("delete database file: %w", err)
		}
	}
	return nil
}
```

Replace with two functions:

```go
// Remove unregisters a ledger. If deleteFile is true and the ledger's DB file
// exists, it is deleted after unregistering. Returns ErrRemoveActive if the
// target is currently the active ledger. The file deletion happens outside
// r.mu — the captured entry path was resolved while holding the lock, and
// blocking all registry reads on a disk-remove call would be needlessly
// pessimistic.
func (r *Registry) Remove(name string, deleteFile bool) error {
	entry, err := r.removeAndSave(name)
	if err != nil {
		return err
	}
	if !deleteFile {
		return nil
	}
	if err := os.Remove(entry.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("delete database file: %w", err)
	}
	return nil
}

// removeAndSave deletes name from r.Ledgers, persists the registry, and
// returns the removed entry so the caller can act on it outside the lock.
// Takes r.mu internally.
func (r *Registry) removeAndSave(name string) (Entry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if name == r.activeNameLocked() {
		return Entry{}, ErrRemoveActive
	}
	entry, exists := r.Ledgers[name]
	if !exists {
		return Entry{}, fmt.Errorf("%w: %q", ErrLedgerNotFound, name)
	}
	delete(r.Ledgers, name)
	if err := r.saveLocked(); err != nil {
		return Entry{}, err
	}
	return entry, nil
}
```

- [ ] **Step 4: Run the existing Add/Switch/Remove tests**

Run: `go test ./internal/ledger/ -v -run 'TestAdd|TestSwitch|TestRemove'`

Expected: all `TestAdd_*`, `TestSwitch_*`, `TestRemove_*` tests PASS — the locking is transparent to single-threaded callers.

If any test fails (especially one involving the active-ledger interaction with `Remove`), STOP and inspect.

---

### Task 5: Lock the read accessors (`Names`, `EntryFor`, `Active`)

**Files:**
- Modify: `internal/ledger/registry.go` — apply locking to `Names` (lines 107–115), `EntryFor` (lines 117–121), `Active` (lines 141–153).

- [ ] **Step 1: Replace `Names`**

Current:

```go
// Names returns all registered ledger names in sorted order.
func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.Ledgers))
	for name := range r.Ledgers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
```

Replace with:

```go
// Names returns all registered ledger names in sorted order. Safe to call
// from external goroutines; takes r.mu internally.
func (r *Registry) Names() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	names := make([]string, 0, len(r.Ledgers))
	for name := range r.Ledgers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
```

- [ ] **Step 2: Replace `EntryFor`**

Current:

```go
// EntryFor returns the Entry for a given name and whether it was found.
func (r *Registry) EntryFor(name string) (Entry, bool) {
	e, ok := r.Ledgers[name]
	return e, ok
}
```

Replace with:

```go
// EntryFor returns the Entry for a given name and whether it was found.
// Safe to call from external goroutines; takes r.mu internally.
func (r *Registry) EntryFor(name string) (Entry, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.Ledgers[name]
	return e, ok
}
```

- [ ] **Step 3: Replace `Active`**

Current:

```go
// Active returns the resolved DB path for the active ledger.
// KEA_LEDGER env var overrides the registry's active field.
func (r *Registry) Active() (string, error) {
	name := r.ActiveName()
	if name == "" {
		return "", ErrNoActiveLedger
	}
	entry, exists := r.Ledgers[name]
	if !exists {
		return "", fmt.Errorf("active ledger %q is not registered — run: kea ledger list", name)
	}
	return entry.Path, nil
}
```

Replace with:

```go
// Active returns the resolved DB path for the active ledger.
// KEA_LEDGER env var overrides the registry's active field. Safe to call
// from external goroutines; takes r.mu internally.
func (r *Registry) Active() (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	name := r.activeNameLocked()
	if name == "" {
		return "", ErrNoActiveLedger
	}
	entry, exists := r.Ledgers[name]
	if !exists {
		return "", fmt.Errorf("active ledger %q is not registered — run: kea ledger list", name)
	}
	return entry.Path, nil
}
```

- [ ] **Step 4: Run the existing accessor tests**

Run: `go test ./internal/ledger/ -v -run 'TestNames|TestEntryFor|TestActive'`

Expected: all tests PASS.

---

### Task 6: Re-run the race tests; confirm they now pass

**Files:** none — verification only.

- [ ] **Step 1: Run the two new race tests under `-race`**

Run: `go test -race ./internal/ledger/ -run 'TestRegistry_ConcurrentAccess|TestRegistry_SwitchVsReload' -v`

Expected: both tests PASS. No panic from concurrent map access. No `DATA RACE` warning from the detector.

If either test still fails, the locking is incomplete — re-audit the touched fields against the spec's audit table.

- [ ] **Step 2: Run the entire ledger package under `-race`**

Run: `go test -race ./internal/ledger/ -v`

Expected: all tests PASS, including the existing `TestWatch_*` family which exercises real fsnotify behavior.

- [ ] **Step 3: Run the entire test suite under `-race` to catch incidental regressions elsewhere**

Run: `go test -race ./...`

Expected: all packages PASS.

If a different package fails under `-race` (e.g., `internal/app` or `internal/api`), capture the failure but do not chase it in this PR — log a follow-up. The spec scopes this work to the registry.

---

### Task 7: Add the `make test-race` target

**Files:**
- Modify: `Makefile` — append a `test-race` target.

The current `Makefile` (verbatim):

```makefile
build:
	go build ./cmd/kea

run:
	go run ./cmd/kea
```

- [ ] **Step 1: Append the `test-race` target**

The new `Makefile`:

```makefile
build:
	go build ./cmd/kea

run:
	go run ./cmd/kea

test-race:
	go test -race ./...
```

Two-space-indented Make targets use tabs for the recipe lines, matching the existing targets. Preserve tab indentation.

- [ ] **Step 2: Verify the target works**

Run: `make test-race`

Expected: same output as `go test -race ./...` from Task 6 Step 3 — all packages PASS.

---

### Task 8: Full-suite verification and commit

**Files:** none — verification and commit only.

- [ ] **Step 1: Run the full test suite without `-race`**

Run: `go test ./...`

Expected: all packages PASS.

- [ ] **Step 2: Run a clean build**

Run: `go build ./...`

Expected: no output, exit 0.

- [ ] **Step 3: Inspect `git status` and stage exactly three files**

Run:

```
git status
```

Verify three modified files (`internal/ledger/registry.go`, `internal/ledger/registry_test.go`, `Makefile`) and the preexisting untracked `docs/web-layer/` directory.

Stage:

```
git add internal/ledger/registry.go \
        internal/ledger/registry_test.go \
        Makefile
```

Verify `git status` shows exactly those three files staged.

- [ ] **Step 4: Commit with HEREDOC**

Use the spec's commit message verbatim:

```
git commit -m "$(cat <<'EOF'
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
- Adds \`make test-race\` so -race becomes a one-liner during development.
EOF
)"
```

This project does not use a `Co-Authored-By` footer (see `git log` on master). Do not add one.

Expected: commit succeeds, working tree clean for tracked files.

---

## Self-Review

**Spec coverage:**

- Spec §"Locking discipline" / read accessors locked: Tasks 3 (`ActiveName`), 5 (`Names`, `EntryFor`, `Active`) ✓
- Spec §"Locking discipline" / mutating methods locked: Task 4 ✓
- Spec §"Locking discipline" / `Save` exported wrapper + private body: Task 2 ✓
- Spec §"saveLocked refactor" / `Load` uses `saveLocked` directly: Task 2 Step 2 ✓
- Spec §"`ActiveName` and the `KEA_LEDGER` env" / `activeNameLocked` helper: Task 3 ✓
- Spec §"Locking discipline" / `Remove`'s post-delete I/O outside the lock: Task 4 Step 3 (`removeAndSave` split) ✓
- Spec §"Race tests" / `TestRegistry_ConcurrentAccess` and `TestRegistry_SwitchVsReload`: Task 1 ✓
- Spec §"Makefile target": Task 7 ✓
- Spec §"Verification" / `go test ./...`, `go test -race ./...`, `go build ./...`: Tasks 6 and 8 ✓
- Spec §"Commit shape" / single commit, scope `(ledger)`, body verbatim, no co-author: Task 8 Step 4 ✓
- Spec §"Out of scope": no tasks attempt save-failure rollback, the `cfg` mutation issue, or `cmd/ledger/*.go` runner changes ✓

**Placeholder scan:** None of the steps contain TBDs, "implement later", or "similar to Task N". Every code change shows the verbatim before/after.

**Type/name consistency:**
- `saveLocked` / `activeNameLocked` / `removeAndSave` used consistently across tasks ✓
- The `Save` exported wrapper is referenced once (Task 2) and not redeclared ✓
- Test names (`TestRegistry_ConcurrentAccess`, `TestRegistry_SwitchVsReload`) match between Task 1 (add), Task 6 (re-run), and the spec ✓
- `fmt.Sprintf("g%d-%d", id, i)` is the only new format string; matches the `g%d-%d` pattern used in the test's name construction ✓
