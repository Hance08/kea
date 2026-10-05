# Wire `Registry.Watch` into `kea serve` — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Spawn `Registry.Watch(ctx)` in a goroutine inside `kea serve`'s `RunE`, so the existing fsnotify watcher actually runs in the server process. Add a regression test that proves the full external-switch-to-store-swap chain works at the App layer.

**Architecture:** ~6 lines added to `cmd/serve.go` (goroutine + error-filtered logging). One new integration test in `internal/app/app_test.go` that uses the real `NewApp` (to wire the OnSwitch callback) and exercises external `Registry.Switch` → fsnotify → callback → store swap → cfg update. Single commit, scope `fix(cmd)`.

**Tech Stack:** Go stdlib (`context`, `errors`, `time`), `fsnotify` (via the existing Registry), `testing`.

**Spec:** [`docs/superpowers/specs/2026-06-07-fix-serve-registry-watcher-wire-design.md`](../specs/2026-06-07-fix-serve-registry-watcher-wire-design.md)

**Note:** This is not strict TDD — the test will pass on first run because all the App-level pieces already work (the bug is purely a missing call in `serve.go`). The test is a regression anchor that proves the integration the production code now relies on.

---

## File Map

- Modify: `cmd/serve.go` — add `context` and `errors` imports; add the watcher goroutine inside `RunE` before `srv := api.NewServer(...)`.
- Modify: `internal/app/app_test.go` — append `TestApp_WatchSwapsStoreOnExternalSwitch` at the end of the file; add `context` and `time` imports.

---

### Task 1: Apply the `cmd/serve.go` change

**Files:**
- Modify: `cmd/serve.go` — current content is 30 lines (verified). Add 2 imports and ~8 lines inside `RunE`.

- [ ] **Step 1: Update the import block**

Current import block (verbatim, lines 5–13):

```go
import (
	"io/fs"
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/hance08/kea/internal/api"
	"github.com/hance08/kea/internal/app"
)
```

Replace with:

```go
import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/hance08/kea/internal/api"
	"github.com/hance08/kea/internal/app"
)
```

(`context` and `errors` added alphabetically at the top of the stdlib block.)

- [ ] **Step 2: Add the watcher goroutine inside `RunE`**

Current `RunE` body (verbatim, lines 19–28):

```go
RunE: func(cmd *cobra.Command, args []string) error {
    logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
    srv := api.NewServer(
        application.Config(),
        application.Service,
        application.Registry,
        migrationFS,
        appDir,
        application.SwitchLedger,
        logger,
    )
    return srv.Run(cmd.Context())
},
```

Replace with:

```go
RunE: func(cmd *cobra.Command, args []string) error {
    logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))

    // Start the registry watcher so external `kea ledger switch` calls
    // made while the server is running cause this server to swap stores.
    // The watcher exits when ctx is cancelled; the app's cleanup also
    // calls StopWatch defensively.
    go func() {
        if err := application.Registry.Watch(cmd.Context()); err != nil &&
            !errors.Is(err, context.Canceled) {
            logger.Error("registry watcher exited", "err", err)
        }
    }()

    srv := api.NewServer(
        application.Config(),
        application.Service,
        application.Registry,
        migrationFS,
        appDir,
        application.SwitchLedger,
        logger,
    )
    return srv.Run(cmd.Context())
},
```

- [ ] **Step 3: Verify the build is clean**

Run: `go build ./...`

Expected: no output, exit 0. If imports are misordered, `gofmt` may complain; run `gofmt -w cmd/serve.go` to normalize and re-build.

- [ ] **Step 4: Verify existing serve tests still pass**

Run: `go test ./cmd/ -run TestNewServeCmdShape -v`

Expected: PASS. The existing test only checks the cobra command shape (`Use`, `Short`, `RunE != nil`) and doesn't exercise the new goroutine.

---

### Task 2: Add the regression test in `internal/app/app_test.go`

**Files:**
- Modify: `internal/app/app_test.go` — add `context` and `time` to imports; append the new test function at the end.

Context: the existing `newTestApp` helper in this file (lines 21–68) constructs an `App` struct directly without going through `NewApp`. That works for the existing tests because they call `a.SwitchLedger("b")` which mutates state inline, not via the OnSwitch callback. **The new test specifically requires the OnSwitch callback to be wired**, so it must use the real `NewApp` constructor (which registers the callback at `internal/app/app.go:50`).

The test simulates the production scenario: server process is running with the watcher up; an external CLI process writes `ledgers.yaml`; the watcher in the server process detects, reloads, fires callback, swaps store, updates cfg.

- [ ] **Step 1: Update the import block in `internal/app/app_test.go`**

Current imports (verbatim, lines 6–16):

```go
import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/hance08/kea/internal/config"
	"github.com/hance08/kea/internal/ledger"
	"github.com/hance08/kea/internal/store"
	"github.com/hance08/kea/migrations"
)
```

Replace with (adds `context` and `time` alphabetically into the stdlib group):

```go
import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hance08/kea/internal/config"
	"github.com/hance08/kea/internal/ledger"
	"github.com/hance08/kea/internal/store"
	"github.com/hance08/kea/migrations"
)
```

Note: `store` may no longer be needed if you confirm the new test doesn't call `store.NewStore` directly. Leave it — existing tests still use it via `newTestApp`.

- [ ] **Step 2: Append the new test function at the end of the file**

Place after the existing `TestSwitchLedger_FailedSwapLeavesStateUnchanged` (which ends around line 147):

```go
// TestApp_WatchSwapsStoreOnExternalSwitch pins the production scenario for
// kea serve: a second process (the CLI) writes ledgers.yaml; the server
// process has a watcher running; fsnotify detects the write; reload() fires
// the OnSwitch callback wired by NewApp; the callback swaps the store and
// updates cfg. This is the chain cmd/serve.go's Watch goroutine relies on.
func TestApp_WatchSwapsStoreOnExternalSwitch(t *testing.T) {
	tempDir := t.TempDir()
	pathA := filepath.Join(tempDir, "a.db")
	pathB := filepath.Join(tempDir, "b.db")

	if err := InitLedgerDB(pathA, migrations.FS); err != nil {
		t.Fatalf("init a: %v", err)
	}
	if err := InitLedgerDB(pathB, migrations.FS); err != nil {
		t.Fatalf("init b: %v", err)
	}

	reg, err := ledger.Load(tempDir)
	if err != nil {
		t.Fatalf("load registry: %v", err)
	}
	delete(reg.Ledgers, "default")
	reg.ActiveLedger = ""
	if err := reg.Add("a", pathA); err != nil {
		t.Fatalf("add a: %v", err)
	}
	if err := reg.Add("b", pathB); err != nil {
		t.Fatalf("add b: %v", err)
	}
	if err := reg.Switch("a"); err != nil {
		t.Fatalf("switch a: %v", err)
	}

	cfg := config.NewDefault()
	cfg.Database.Path = pathA
	cfg.ActiveLedger = "a"

	// Use NewApp so the OnSwitch callback (app.go:50) is wired. The existing
	// newTestApp helper bypasses NewApp and doesn't register the callback.
	a, cleanup, err := NewApp(cfg, reg, migrations.FS)
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	t.Cleanup(cleanup)

	// Spawn the watcher in a goroutine, mimicking what cmd/serve.go does.
	ctx, cancel := context.WithCancel(t.Context())
	watchDone := make(chan struct{})
	go func() {
		defer close(watchDone)
		_ = a.Registry.Watch(ctx)
	}()

	// Let fsnotify install its watch — same pattern as the existing
	// TestWatch_* tests in internal/ledger/registry_test.go.
	time.Sleep(200 * time.Millisecond)

	// External writer: a second Registry instance loads ledgers.yaml fresh
	// (simulating a separate CLI process), then calls Switch which writes
	// the file. fsnotify in the first process picks up the write, reload()
	// fires the OnSwitch callback, the callback swaps the store and updates cfg.
	extReg, err := ledger.Load(tempDir)
	if err != nil {
		t.Fatalf("load external registry: %v", err)
	}
	if err := extReg.Switch("b"); err != nil {
		t.Fatalf("external switch: %v", err)
	}

	// Poll for the callback to fire. Debounce is 100ms; allow generous timeout
	// for slow CI environments.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if a.Config().ActiveLedger == "b" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if a.Config().ActiveLedger != "b" {
		t.Fatalf("ActiveLedger: got %q, want %q (external switch did not propagate)",
			a.Config().ActiveLedger, "b")
	}
	if a.Config().Database.Path != pathB {
		t.Errorf("Database.Path: got %q, want %q after external switch",
			a.Config().Database.Path, pathB)
	}

	// Cancel and confirm the watcher exits cleanly.
	cancel()
	select {
	case <-watchDone:
		// OK
	case <-time.After(2 * time.Second):
		t.Fatal("watcher goroutine did not exit after context cancel")
	}
}
```

- [ ] **Step 3: Run the new test**

Run: `go test ./internal/app/ -run TestApp_WatchSwapsStoreOnExternalSwitch -v`

Expected: PASS. If it FAILS with "ActiveLedger: got "a", want "b"" — the OnSwitch callback isn't firing. Possible causes:
- The watcher didn't install in time (increase the initial sleep to 400ms).
- fsnotify isn't supported on the test platform (rare on macOS/Linux).
- `NewApp`'s callback registration silently broke.

If it FAILS with "watcher goroutine did not exit after context cancel" — the watcher's `select` loop isn't honoring ctx.Done(). Re-check `internal/ledger/registry.go::Watch`.

- [ ] **Step 4: Run with the race detector**

Run: `go test -race ./internal/app/ -run TestApp_WatchSwapsStoreOnExternalSwitch -v`

Expected: PASS. The test exercises:
- The watcher goroutine reading from `r.callbacks` (post-PR-#181 hardening).
- The OnSwitch callback writing `cfg.ActiveLedger` and `cfg.Database.Path` from the watcher goroutine.
- The main test goroutine reading `a.Config().ActiveLedger` in the poll loop.

The cfg writes vs reads on different goroutines is the deferred `app.cfg` race (out of scope per the spec). It *may* surface here under `-race` because the test deliberately exercises it. If `-race` flags this, document it in your report and proceed — the production fix is the deferred chip, not this PR.

- [ ] **Step 5: Run the full app suite to catch regressions**

Run: `go test ./internal/app/ -v`

Expected: all 4 tests pass (3 existing + 1 new). The existing tests use `newTestApp` which doesn't go through `NewApp`, so they shouldn't be affected by anything in this commit.

---

### Task 3: Full verification and commit

**Files:** none — verification and commit only.

- [ ] **Step 1: Run the full Go test suite**

Run: `go test ./...`

Expected: all packages pass.

- [ ] **Step 2: Run under `-race`**

Run: `go test -race ./...`

Expected: green. If the new test trips `-race` on the cfg fields, document it (out of scope per spec; deferred chip).

- [ ] **Step 3: Clean build**

Run: `go build ./...`

Expected: no output, exit 0.

- [ ] **Step 4: Manual smoke test (recommended but optional)**

In one terminal: `make run`.
In another: `curl -s http://localhost:8080/api/balances | jq '.items[0].name'` (note the current ledger's first account).
In a third: `go run ./cmd/kea ledger switch <other-ledger-name>`.
Back to the second terminal: `curl -s http://localhost:8080/api/balances | jq '.items[0].name'` — should now reflect the other ledger's first account, not the original.

If the second curl still returns the original ledger's data, the fix didn't take effect — STOP and investigate before committing. Confirm `cmd/serve.go` was actually edited (the build may have been cached).

- [ ] **Step 5: Stage and commit**

Run `git status`. Expected modified files:
- `cmd/serve.go`
- `internal/app/app_test.go`

The untracked `docs/web-layer/` directory stays untracked.

Stage:

```
git add cmd/serve.go internal/app/app_test.go
```

Commit with HEREDOC:

```
git commit -m "$(cat <<'EOF'
fix(cmd): start registry watcher in kea serve

cmd/serve.go never called Registry.Watch(ctx), so the fsnotify watcher
hardened in PR #181 never ran in the server process. External
`kea ledger switch` writes updated ledgers.yaml but the running server
had no listener and kept serving the old DB — issue #119's split-brain
materialized in practice.

Spawn Watch in a goroutine bound to the server's context. The watcher
exits on context cancellation; app cleanup calls StopWatch defensively
for double-safety. The OnSwitch callback wired in app.go:50 then swaps
the store and updates cfg, so the very next request hits the new DB.

Adds TestApp_WatchSwapsStoreOnExternalSwitch covering the full chain:
external file mutation → fsnotify → reload → callback → swap.
EOF
)"
```

This project does not use a `Co-Authored-By` footer (verify with `git log -3 --format=%B`). Do not add one.

Expected: commit succeeds, working tree clean for tracked files.

---

## Self-Review

**Spec coverage:**

- Spec §"Change / `cmd/serve.go`" (goroutine + error-filtered logging): Task 1 ✓
- Spec §"Test" (`TestApp_WatchSwapsStoreOnExternalSwitch` with full chain — external write → fsnotify → callback → swap → cfg update): Task 2 ✓
- Spec §"Verification" (`go test ./...`, `go test -race ./...`, `go build ./...`, manual smoke): Task 3 ✓
- Spec §"Commit shape" (single commit, `fix(cmd):` scope, verbatim body, no co-author): Task 3 Step 5 ✓
- Spec §"Out of scope" (deferred `app.cfg` race remains the chip's territory; no retry on `fsnotify.NewWatcher` failure; no #119 doc edits): no tasks attempt any of these ✓

**Placeholder scan:**

- No TBDs, no "implement later", no "similar to Task N".
- Every code step shows the verbatim before/after.
- Expected failure modes in Task 2 Step 3 are concrete.
- The TDD-vs-regression-test framing is documented up front in the plan header note.

**Type/name consistency:**

- `application.Registry.Watch(cmd.Context())` in Task 1 matches the signature `Watch(ctx context.Context) error` (verified in `internal/ledger/registry.go:269`).
- `errors.Is(err, context.Canceled)` filter matches what `Watch` returns when its `<-ctx.Done()` branch fires (verified via `return ctx.Err()` in the watcher's select loop).
- `NewApp(cfg, reg, migrations.FS)` in Task 2 matches the signature `(cfg *config.Config, registry *ledger.Registry, migrationFS fs.FS) (*App, func(), error)` in `app.go`.
- `a.Config().ActiveLedger` / `a.Config().Database.Path` access pattern matches the existing `newTestApp`-based tests' assertion shape (`a.cfg.ActiveLedger` in those — same field, exposed via the public `Config()` method in the new test).
- `t.Context()` requires Go ≥ 1.24; project is on Go 1.25 (verified in prior PRs this session). Safe.
