# Web API Ledgers Endpoints — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add five REST endpoints (`GET /api/ledgers`, `GET /api/ledgers/active`, `POST /api/ledgers`, `POST /api/ledgers/switch`, `DELETE /api/ledgers/{name}`) so the SPA can manage ledgers without dropping to the CLI.

**Architecture:** A new `App.SwitchLedger(name)` method does the synchronous store swap. `internal/api/Server` grows four fields — registry, migrations, appDir, switchFn — wired from `cmd/serve.go`. Handlers live in `internal/api/ledgers.go` and call the registry or the switch function directly; no new service-layer methods. Error mapping gains four `errors.Is` branches for the existing `ledger.*` sentinels.

**Tech Stack:** Go, chi v5 router, `httptest` + stdlib for tests. Reuses everything from the foundation/read/write specs.

**Spec:** `docs/superpowers/specs/2026-06-04-web-api-ledgers-design.md`

**Conventions to follow throughout the plan:**
- Every file starts with the SPDX/copyright header used by the rest of the repo:
  ```go
  // SPDX-License-Identifier: GPL-3.0-or-later
  // Copyright (C) 2026  Hance Chin
  ```
- Tests use stdlib (`testing`, `net/http/httptest`); follow the table-driven shape used in `internal/api/errors_test.go`.
- Commits are conventional: `feat(api): ...`, `test(api): ...`, `refactor(app): ...`. Match the style in recent commit history (`git log --oneline -20`).
- After each task, run `go build ./...` and the targeted `go test ./...` subset shown in the task; full `go test ./...` runs once at the end.

---

## File Structure

**New files:**
- `internal/api/ledgers.go` — five HTTP handlers + DTOs (`ledgerInfo`, `ledgerListResponse`, `createLedgerRequest`, `switchLedgerRequest`) + `validateLedgerName`. ~150 LOC.
- `internal/api/ledgers_test.go` — handler tests, table-driven. ~250 LOC.
- `internal/app/app_test.go` — `TestSwitchLedger*` over a real `store.Store`. ~120 LOC.

**Modified files:**
- `internal/app/app.go` — add `store`, `migrations`, `cfg` fields to `App`; add `SwitchLedger` method.
- `internal/api/server.go` — extend `Server` struct and `NewServer` signature.
- `internal/api/router.go` — register five new routes.
- `internal/api/errors.go` — four new `errors.Is` branches + two new error codes (`cannot_remove_active`, `no_active_ledger`).
- `internal/api/errors_test.go` — four new table rows + four wrapped-with-`%w` rows.
- `internal/api/testhelper_test.go` — update `newServerForWrite` to the new `NewServer` signature; add `newTestServerWithLedger` helper.
- `internal/api/server_test.go`, `internal/api/router_test.go` — update `NewServer` calls to the new signature (pass nil/zero values for unused new args).
- `cmd/serve.go` — pass the new args into `NewServer`.

**Unchanged:** service layer, repository, store internals, model, migrations, config struct, CLI `cmd/ledger/*`.

---

## Task Ordering Rationale

We work bottom-up so each task leaves the tree compiling and tests green:

1. `App.SwitchLedger` first — pure new code, no signature breakage.
2. `Server` signature change second — touches every NewServer caller in one task so the build stays green.
3. Error mapping third — adds branches without affecting existing routes.
4. Test substrate fourth — adds the helper that handler tests need.
5. Each of the five handlers gets its own task: failing test → implementation → router wiring → commit.
6. Final verification task.

---

## Task 1: `App.SwitchLedger`

**Files:**
- Modify: `internal/app/app.go`
- Create: `internal/app/app_test.go`

- [ ] **Step 1.1: Add the three new fields to `App` and capture them in `NewApp`.**

In `internal/app/app.go`, expand the `App` struct and capture the three values inside `NewApp` so `SwitchLedger` has what it needs. Current struct (around line 19):

```go
type App struct {
    Service  *service.Service
    Registry *ledger.Registry
}
```

Change to (keeping field order such that existing field accesses still work — `Service` and `Registry` stay exported, the new ones are unexported because they're internal wiring):

```go
type App struct {
    Service    *service.Service
    Registry   *ledger.Registry
    store      *store.Store
    migrations fs.FS
    cfg        *config.Config
}
```

Then inside `NewApp`, the `return &App{...}` (currently around line 63) becomes:

```go
return &App{
    Service:    svc,
    Registry:   registry,
    store:      dbStore,
    migrations: migrationFS,
    cfg:        cfg,
}, cleanup, nil
```

No other code in `NewApp` changes.

- [ ] **Step 1.2: Run the build to confirm the struct change compiles.**

Run: `go build ./...`
Expected: no output, exit 0. No other files reference the new fields yet, so this is purely a struct extension.

- [ ] **Step 1.3: Write the failing test for `SwitchLedger` success.**

Create `internal/app/app_test.go`:

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package app

import (
    "errors"
    "path/filepath"
    "testing"

    "github.com/hance08/kea/internal/config"
    "github.com/hance08/kea/internal/ledger"
    "github.com/hance08/kea/internal/store"
    "github.com/hance08/kea/migrations"
)

// newTestApp returns an *App wired to two on-disk ledgers ("a", "b"), both
// registered, with "a" active. tempDir holds ledgers.yaml plus the two DB
// files.
func newTestApp(t *testing.T) (a *App, tempDir string, pathA, pathB string) {
    t.Helper()
    tempDir = t.TempDir()
    pathA = filepath.Join(tempDir, "a.db")
    pathB = filepath.Join(tempDir, "b.db")

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
    // ledger.Load auto-seeds a "default" entry; drop it so the test starts
    // with exactly "a" and "b".
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

    st, err := store.NewStore(pathA, migrations.FS)
    if err != nil {
        t.Fatalf("new store: %v", err)
    }
    t.Cleanup(func() { _ = st.Close() })

    return &App{
        Registry:   reg,
        store:      st,
        migrations: migrations.FS,
        cfg:        cfg,
    }, tempDir, pathA, pathB
}

func TestSwitchLedger_SwapsStoreAndUpdatesConfig(t *testing.T) {
    a, _, _, pathB := newTestApp(t)

    if err := a.SwitchLedger("b"); err != nil {
        t.Fatalf("SwitchLedger: %v", err)
    }

    if a.cfg.ActiveLedger != "b" {
        t.Errorf("cfg.ActiveLedger: got %q, want %q", a.cfg.ActiveLedger, "b")
    }
    if a.cfg.Database.Path != pathB {
        t.Errorf("cfg.Database.Path: got %q, want %q", a.cfg.Database.Path, pathB)
    }
    if a.Registry.ActiveLedger != "b" {
        t.Errorf("registry.ActiveLedger: got %q, want %q", a.Registry.ActiveLedger, "b")
    }

    // Verify the store now talks to pathB by writing through the swapped
    // connection.
    _, err := a.store.DB().ExecContext(t.Context(),
        "INSERT INTO accounts (name, type, currency) VALUES (?, ?, ?)",
        "Assets:Probe", "A", "USD",
    )
    if err != nil {
        t.Fatalf("insert into swapped store: %v", err)
    }
}

func TestSwitchLedger_UnknownNameReturnsErrLedgerNotFound(t *testing.T) {
    a, _, pathA, _ := newTestApp(t)

    err := a.SwitchLedger("nope")
    if !errors.Is(err, ledger.ErrLedgerNotFound) {
        t.Fatalf("err: got %v, want ErrLedgerNotFound", err)
    }
    if a.cfg.ActiveLedger != "a" {
        t.Errorf("cfg.ActiveLedger leaked: got %q, want %q", a.cfg.ActiveLedger, "a")
    }
    if a.cfg.Database.Path != pathA {
        t.Errorf("cfg.Database.Path leaked: got %q, want %q", a.cfg.Database.Path, pathA)
    }
    if a.Registry.ActiveLedger != "a" {
        t.Errorf("registry.ActiveLedger leaked: got %q, want %q", a.Registry.ActiveLedger, "a")
    }
}

func TestSwitchLedger_FailedSwapLeavesStateUnchanged(t *testing.T) {
    a, tempDir, pathA, _ := newTestApp(t)

    // Register a ledger whose path points at a non-existent directory; Swap
    // will fail before any state mutates. The corresponding store-layer
    // behavior is pinned by store.TestSwap_FailedSwapKeepsOldConnection.
    badPath := filepath.Join(tempDir, "no-such-dir", "bad.db")
    if err := a.Registry.Add("bad", badPath); err != nil {
        t.Fatalf("add bad: %v", err)
    }

    err := a.SwitchLedger("bad")
    if err == nil {
        t.Fatal("expected error from SwitchLedger to nonexistent path")
    }
    if a.cfg.ActiveLedger != "a" {
        t.Errorf("cfg.ActiveLedger leaked: got %q, want %q", a.cfg.ActiveLedger, "a")
    }
    if a.cfg.Database.Path != pathA {
        t.Errorf("cfg.Database.Path leaked: got %q, want %q", a.cfg.Database.Path, pathA)
    }
    if a.Registry.ActiveLedger != "a" {
        t.Errorf("registry.ActiveLedger leaked: got %q, want %q", a.Registry.ActiveLedger, "a")
    }
}
```

- [ ] **Step 1.4: Run the tests to verify they fail.**

Run: `go test ./internal/app/ -run TestSwitchLedger -v`
Expected: compilation failure — `a.SwitchLedger undefined`.

- [ ] **Step 1.5: Implement `SwitchLedger`.**

Add at the bottom of `internal/app/app.go`:

```go
// SwitchLedger atomically swaps the active store to the named ledger,
// updates the in-memory config, and persists the active-name change to
// ledgers.yaml. The registry's fsnotify watcher subsequently fires reload(),
// which short-circuits because ActiveLedger already matches.
func (a *App) SwitchLedger(name string) error {
    entry, ok := a.Registry.EntryFor(name)
    if !ok {
        return fmt.Errorf("%w: %q", ledger.ErrLedgerNotFound, name)
    }
    if err := a.store.Swap(entry.Path, a.migrations); err != nil {
        return fmt.Errorf("swap store: %w", err)
    }
    a.cfg.ActiveLedger = name
    a.cfg.Database.Path = entry.Path
    return a.Registry.Switch(name)
}
```

`fmt` is already imported in `app.go`; no import changes needed.

- [ ] **Step 1.6: Run the tests to verify they pass.**

Run: `go test ./internal/app/ -run TestSwitchLedger -v`
Expected: all three `TestSwitchLedger_*` tests PASS.

- [ ] **Step 1.7: Run the full app + store test packages to verify nothing else broke.**

Run: `go test ./internal/app/... ./internal/store/... ./internal/ledger/...`
Expected: PASS across all three packages.

- [ ] **Step 1.8: Commit.**

```bash
git add internal/app/app.go internal/app/app_test.go
git commit -m "$(cat <<'EOF'
feat(app): add App.SwitchLedger for synchronous ledger swaps

Wires the API-initiated ledger switch path: resolve the new DB
from the registry, swap the store, update cfg, then persist the
active name. The fsnotify reload becomes a no-op because
ActiveLedger already matches by the time the watcher fires.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 2: Extend `NewServer` signature

This task touches every `NewServer` caller in one shot to keep the build green.

**Files:**
- Modify: `internal/api/server.go`
- Modify: `internal/api/testhelper_test.go`
- Modify: `internal/api/router_test.go`
- Modify: `internal/api/server_test.go`
- Modify: `cmd/serve.go`

- [ ] **Step 2.1: Extend `Server` struct and `NewServer`.**

In `internal/api/server.go`, update imports to add `io/fs` and `github.com/hance08/kea/internal/ledger`:

```go
import (
    "context"
    "errors"
    "io/fs"
    "log/slog"
    "net"
    "net/http"
    "strconv"
    "time"

    "github.com/hance08/kea/internal/config"
    "github.com/hance08/kea/internal/ledger"
    "github.com/hance08/kea/internal/service"
)
```

Replace the struct and constructor:

```go
type Server struct {
    cfg        *config.Config
    svc        *service.Service
    registry   *ledger.Registry
    migrations fs.FS
    appDir     string
    switchFn   func(name string) error
    logger     *slog.Logger
    http       *http.Server
}

func NewServer(
    cfg *config.Config,
    svc *service.Service,
    registry *ledger.Registry,
    migrations fs.FS,
    appDir string,
    switchFn func(name string) error,
    logger *slog.Logger,
) *Server {
    s := &Server{
        cfg:        cfg,
        svc:        svc,
        registry:   registry,
        migrations: migrations,
        appDir:     appDir,
        switchFn:   switchFn,
        logger:     logger,
    }
    addr := net.JoinHostPort(cfg.Server.Host, strconv.Itoa(cfg.Server.Port))
    s.http = &http.Server{
        Addr:              addr,
        Handler:           s.routes(),
        ReadHeaderTimeout: 5 * time.Second,
    }
    return s
}
```

`Run` stays unchanged.

- [ ] **Step 2.2: Update `cmd/serve.go`.**

Replace the file's body with:

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package cmd

import (
    "io/fs"
    "log/slog"
    "os"

    "github.com/spf13/cobra"

    "github.com/hance08/kea/internal/api"
    "github.com/hance08/kea/internal/app"
)

func NewServeCmd(application *app.App, migrationFS fs.FS, appDir string) *cobra.Command {
    return &cobra.Command{
        Use:   "serve",
        Short: "Run the local web server",
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
    }
}
```

This signature drops `*service.Service` and `*config.Config` in favour of `*app.App`, since the API server now needs more of App's state and reaching through one accessor is cleaner than passing five things.

- [ ] **Step 2.3: Add a `Config()` accessor on `*app.App`.**

`app.App` currently exposes `Service` and `Registry` as fields; `cfg` is unexported (added in Task 1). For `cmd/serve.go` to pass the config through, add a tiny accessor at the bottom of `internal/app/app.go`:

```go
// Config returns the config in use. Used by callers that build subcommands
// from *App and need the same cfg pointer NewApp captured.
func (a *App) Config() *config.Config { return a.cfg }
```

- [ ] **Step 2.4: Update `cmd/root.go` to pass `*app.App` to `NewServeCmd`.**

Find the existing line (around line 151):

```go
rootCmd.AddCommand(NewServeCmd(application.Service, cfg))
```

Replace with:

```go
rootCmd.AddCommand(NewServeCmd(application, migrations, appDir))
```

`migrations` and `appDir` are already in scope here (see lines 88 / 100 of `cmd/root.go`).

- [ ] **Step 2.5: Update `internal/api/testhelper_test.go`.**

In `newServerForWrite` (the only caller of `NewServer` in the helper), update line 84:

Before:
```go
srv := NewServer(cfg, svc, discardLogger())
```

After:
```go
srv := NewServer(cfg, svc, nil, nil, "", nil, discardLogger())
```

Passing `nil` for `registry`, `migrations`, `switchFn` and `""` for `appDir` is fine because every existing test in `accounts_test.go`, `transactions_test.go`, etc. exercises handlers that don't touch ledger state. The new ledger-aware helper is introduced in Task 4.

- [ ] **Step 2.6: Update `internal/api/router_test.go` and `internal/api/server_test.go`.**

Both files contain the same call shape:

Before:
```go
srv := NewServer(cfg, nil, discardLogger())
```

After:
```go
srv := NewServer(cfg, nil, nil, nil, "", nil, discardLogger())
```

- [ ] **Step 2.7: Run the build to verify everything compiles.**

Run: `go build ./...`
Expected: no errors.

- [ ] **Step 2.8: Run the full test suite to verify nothing regressed.**

Run: `go test ./...`
Expected: PASS across all packages (the new ledger handlers don't exist yet, but every existing test should still pass).

- [ ] **Step 2.9: Commit.**

```bash
git add internal/api/server.go internal/api/testhelper_test.go \
        internal/api/router_test.go internal/api/server_test.go \
        internal/app/app.go cmd/serve.go cmd/root.go
git commit -m "$(cat <<'EOF'
refactor(api): extend NewServer with registry, migrations, appDir, switchFn

Threads the dependencies the upcoming ledger handlers need. NewServeCmd
now takes *app.App so it can reach Service, Registry, Config, and the
new SwitchLedger method without an ever-growing arg list.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: Error mapping for the four ledger sentinels

**Files:**
- Modify: `internal/api/errors_test.go`
- Modify: `internal/api/errors.go`

- [ ] **Step 3.1: Add failing rows to `TestMapError`.**

Update the `import` block in `internal/api/errors_test.go` to add the ledger import:

```go
import (
    "errors"
    "fmt"
    "testing"

    "github.com/hance08/kea/internal/ledger"
    "github.com/hance08/kea/internal/service"
)
```

Append eight new rows to the `cases` slice (immediately before the `unknown` row):

```go
{"ledger_not_found", ledger.ErrLedgerNotFound, 404, "not_found", ""},
{"ledger_not_found_wrapped", fmt.Errorf("ledger %q: %w", "x", ledger.ErrLedgerNotFound), 404, "not_found", ""},
{"ledger_exists", ledger.ErrLedgerExists, 409, "already_exists", ""},
{"ledger_exists_wrapped", fmt.Errorf("ledger %q: %w", "x", ledger.ErrLedgerExists), 409, "already_exists", ""},
{"remove_active", ledger.ErrRemoveActive, 409, "cannot_remove_active", ""},
{"remove_active_wrapped", fmt.Errorf("active: %w", ledger.ErrRemoveActive), 409, "cannot_remove_active", ""},
{"no_active_ledger", ledger.ErrNoActiveLedger, 404, "no_active_ledger", ""},
{"no_active_ledger_wrapped", fmt.Errorf("ledger: %w", ledger.ErrNoActiveLedger), 404, "no_active_ledger", ""},
```

- [ ] **Step 3.2: Run the tests to verify they fail.**

Run: `go test ./internal/api/ -run TestMapError -v`
Expected: the eight new rows FAIL with `status: got 500, want 404/409`.

- [ ] **Step 3.3: Implement the new mapping branches.**

In `internal/api/errors.go`:

1. Add the ledger import:

```go
import (
    "errors"
    "net/http"

    "github.com/hance08/kea/internal/ledger"
    "github.com/hance08/kea/internal/service"
)
```

2. Add four cases to `mapError`'s switch, immediately before the `default` branch:

```go
case errors.Is(err, ledger.ErrLedgerNotFound):
    return http.StatusNotFound, errorBody{Error: "not_found", Message: err.Error()}
case errors.Is(err, ledger.ErrLedgerExists):
    return http.StatusConflict, errorBody{Error: "already_exists", Message: err.Error()}
case errors.Is(err, ledger.ErrRemoveActive):
    return http.StatusConflict, errorBody{Error: "cannot_remove_active", Message: err.Error()}
case errors.Is(err, ledger.ErrNoActiveLedger):
    return http.StatusNotFound, errorBody{Error: "no_active_ledger", Message: err.Error()}
```

- [ ] **Step 3.4: Run the tests to verify they pass.**

Run: `go test ./internal/api/ -run TestMapError -v`
Expected: all rows PASS.

- [ ] **Step 3.5: Commit.**

```bash
git add internal/api/errors.go internal/api/errors_test.go
git commit -m "$(cat <<'EOF'
feat(api): map ledger sentinels to HTTP status codes

Adds errors.Is branches for ErrLedgerNotFound (404), ErrLedgerExists
(409), ErrRemoveActive (409, code "cannot_remove_active"), and
ErrNoActiveLedger (404, code "no_active_ledger"). Wrapped-error rows
pin the errors.Is propagation.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: Test substrate — `newTestServerWithLedger`

**Files:**
- Modify: `internal/api/testhelper_test.go`

- [ ] **Step 4.1: Add the new helper.**

Append to `internal/api/testhelper_test.go`:

```go
// newTestServerWithLedger wires a fresh Server backed by a real on-disk
// *ledger.Registry plus a fake switchFn that records calls and updates the
// registry's active name without invoking dbStore.Swap.
//
// The real store-swap behavior is exercised in internal/app/app_test.go;
// duplicating it at the HTTP boundary does not pin additional behavior.
func newTestServerWithLedger(t *testing.T) (
    ts *httptest.Server,
    reg *ledger.Registry,
    appDir string,
    switchCalls *[]string,
) {
    t.Helper()

    appDir = t.TempDir()

    var err error
    reg, err = ledger.Load(appDir)
    if err != nil {
        t.Fatalf("ledger.Load: %v", err)
    }

    // ledger.Load auto-creates a "default" entry pointing at <appDir>/kea.db
    // and sets it active. Remove it so tests start from a clean slate; tests
    // that need a default-active ledger seed one explicitly.
    delete(reg.Ledgers, "default")
    reg.ActiveLedger = ""
    if err := reg.Save(); err != nil {
        t.Fatalf("clear default: %v", err)
    }

    calls := []string{}
    switchLedger := func(name string) error {
        calls = append(calls, name)
        if _, ok := reg.EntryFor(name); !ok {
            return fmt.Errorf("%w: %q", ledger.ErrLedgerNotFound, name)
        }
        return reg.Switch(name)
    }

    cfg := config.NewDefault()
    srv := NewServer(cfg, nil, reg, migrations.FS, appDir, switchLedger, discardLogger())
    ts = httptest.NewServer(srv.routes())
    t.Cleanup(ts.Close)

    return ts, reg, appDir, &calls
}
```

Add the missing imports at the top:

```go
import (
    "fmt"
    "net/http/httptest"
    "path/filepath"
    "testing"

    "github.com/hance08/kea/internal/config"
    "github.com/hance08/kea/internal/ledger"
    "github.com/hance08/kea/internal/model"
    "github.com/hance08/kea/internal/service"
    "github.com/hance08/kea/internal/store"
    "github.com/hance08/kea/migrations"
)
```

(`filepath` was already imported; `fmt` and `ledger` are new.)

- [ ] **Step 4.2: Add a smoke test for the helper.**

Append to `internal/api/testhelper_test.go`:

```go
func TestNewTestServerWithLedger_StartsClean(t *testing.T) {
    _, reg, appDir, calls := newTestServerWithLedger(t)
    if reg == nil {
        t.Fatal("registry is nil")
    }
    if got := reg.ActiveName(); got != "" {
        t.Errorf("ActiveName: got %q, want \"\"", got)
    }
    if len(reg.Names()) != 0 {
        t.Errorf("Names: got %v, want empty", reg.Names())
    }
    if appDir == "" {
        t.Error("appDir is empty")
    }
    if calls == nil {
        t.Error("switchCalls slice is nil")
    }
}
```

- [ ] **Step 4.3: Run the smoke test.**

Run: `go test ./internal/api/ -run TestNewTestServerWithLedger -v`
Expected: PASS.

- [ ] **Step 4.4: Commit.**

```bash
git add internal/api/testhelper_test.go
git commit -m "$(cat <<'EOF'
test(api): add newTestServerWithLedger helper

Wires a fresh Server with a real on-disk Registry rooted in t.TempDir
plus a fake switchFn that updates the registry without invoking
dbStore.Swap. Used by the upcoming ledger handler tests.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 5: `handleListLedgers` + `GET /api/ledgers`

**Files:**
- Create: `internal/api/ledgers.go`
- Create: `internal/api/ledgers_test.go`
- Modify: `internal/api/router.go`

- [ ] **Step 5.1: Write the failing test.**

Create `internal/api/ledgers_test.go`:

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import (
    "bytes"
    "encoding/json"
    "io"
    "net/http"
    "path/filepath"
    "testing"
)

func getJSON(t *testing.T, url string) (int, []byte) {
    t.Helper()
    resp, err := http.Get(url)
    if err != nil {
        t.Fatalf("GET %s: %v", url, err)
    }
    defer resp.Body.Close()
    body, err := io.ReadAll(resp.Body)
    if err != nil {
        t.Fatalf("read body: %v", err)
    }
    return resp.StatusCode, body
}

func postJSON(t *testing.T, url string, payload any) (int, []byte) {
    t.Helper()
    var buf bytes.Buffer
    if payload != nil {
        if err := json.NewEncoder(&buf).Encode(payload); err != nil {
            t.Fatalf("encode payload: %v", err)
        }
    }
    resp, err := http.Post(url, "application/json", &buf)
    if err != nil {
        t.Fatalf("POST %s: %v", url, err)
    }
    defer resp.Body.Close()
    body, err := io.ReadAll(resp.Body)
    if err != nil {
        t.Fatalf("read body: %v", err)
    }
    return resp.StatusCode, body
}

func deleteURL(t *testing.T, url string) (int, []byte) {
    t.Helper()
    req, err := http.NewRequest(http.MethodDelete, url, nil)
    if err != nil {
        t.Fatalf("new request: %v", err)
    }
    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        t.Fatalf("DELETE %s: %v", url, err)
    }
    defer resp.Body.Close()
    body, err := io.ReadAll(resp.Body)
    if err != nil {
        t.Fatalf("read body: %v", err)
    }
    return resp.StatusCode, body
}

func TestHandleListLedgers_Empty(t *testing.T) {
    ts, _, _, _ := newTestServerWithLedger(t)

    status, body := getJSON(t, ts.URL+"/api/ledgers")
    if status != http.StatusOK {
        t.Fatalf("status: got %d, want 200; body=%s", status, body)
    }

    var got struct {
        Active string `json:"active"`
        Items  []struct {
            Name   string `json:"name"`
            Path   string `json:"path"`
            Active bool   `json:"active"`
        } `json:"items"`
    }
    if err := json.Unmarshal(body, &got); err != nil {
        t.Fatalf("unmarshal: %v; body=%s", err, body)
    }
    if got.Active != "" {
        t.Errorf("active: got %q, want \"\"", got.Active)
    }
    if len(got.Items) != 0 {
        t.Errorf("items: got %v, want []", got.Items)
    }
}

func TestHandleListLedgers_TwoRegisteredOneActive(t *testing.T) {
    ts, reg, appDir, _ := newTestServerWithLedger(t)

    pathA := filepath.Join(appDir, "ledgers", "alpha.db")
    pathB := filepath.Join(appDir, "ledgers", "beta.db")
    if err := reg.Add("alpha", pathA); err != nil {
        t.Fatalf("add alpha: %v", err)
    }
    if err := reg.Add("beta", pathB); err != nil {
        t.Fatalf("add beta: %v", err)
    }
    if err := reg.Switch("alpha"); err != nil {
        t.Fatalf("switch alpha: %v", err)
    }

    status, body := getJSON(t, ts.URL+"/api/ledgers")
    if status != http.StatusOK {
        t.Fatalf("status: got %d, want 200; body=%s", status, body)
    }

    var got struct {
        Active string `json:"active"`
        Items  []struct {
            Name   string `json:"name"`
            Path   string `json:"path"`
            Active bool   `json:"active"`
        } `json:"items"`
    }
    if err := json.Unmarshal(body, &got); err != nil {
        t.Fatalf("unmarshal: %v; body=%s", err, body)
    }
    if got.Active != "alpha" {
        t.Errorf("active: got %q, want %q", got.Active, "alpha")
    }
    if len(got.Items) != 2 {
        t.Fatalf("items: got %d, want 2", len(got.Items))
    }
    // Items must be sorted by name: alpha, beta.
    if got.Items[0].Name != "alpha" || got.Items[1].Name != "beta" {
        t.Errorf("sort order: got [%q, %q], want [alpha, beta]",
            got.Items[0].Name, got.Items[1].Name)
    }
    if !got.Items[0].Active || got.Items[1].Active {
        t.Errorf("active flags: got [%v, %v], want [true, false]",
            got.Items[0].Active, got.Items[1].Active)
    }
    if got.Items[0].Path != pathA || got.Items[1].Path != pathB {
        t.Errorf("paths: got [%q, %q], want [%q, %q]",
            got.Items[0].Path, got.Items[1].Path, pathA, pathB)
    }
}
```

- [ ] **Step 5.2: Run the tests to verify they fail.**

Run: `go test ./internal/api/ -run TestHandleListLedgers -v`
Expected: 404 (route not registered yet) — the assertions on `status` fail.

- [ ] **Step 5.3: Create `internal/api/ledgers.go` with the DTOs and the list handler.**

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import (
    "net/http"
)

type ledgerInfo struct {
    Name   string `json:"name"`
    Path   string `json:"path"`
    Active bool   `json:"active"`
}

type ledgerListResponse struct {
    Active string       `json:"active"`
    Items  []ledgerInfo `json:"items"`
}

type createLedgerRequest struct {
    Name string `json:"name"`
}

type switchLedgerRequest struct {
    Name string `json:"name"`
}

func (s *Server) handleListLedgers(w http.ResponseWriter, r *http.Request) error {
    active := s.registry.ActiveName()
    names := s.registry.Names()
    items := make([]ledgerInfo, 0, len(names))
    for _, n := range names {
        e, _ := s.registry.EntryFor(n)
        items = append(items, ledgerInfo{Name: n, Path: e.Path, Active: n == active})
    }
    return writeJSON(w, http.StatusOK, ledgerListResponse{Active: active, Items: items})
}
```

- [ ] **Step 5.4: Register the route in `internal/api/router.go`.**

Append inside the `r.Route("/api", ...)` block, after the last existing line (`/reports/net-worth`):

```go
r.Method(http.MethodGet, "/ledgers", apiHandler(s.handleListLedgers))
```

- [ ] **Step 5.5: Run the tests to verify they pass.**

Run: `go test ./internal/api/ -run TestHandleListLedgers -v`
Expected: PASS for both tests.

- [ ] **Step 5.6: Commit.**

```bash
git add internal/api/ledgers.go internal/api/ledgers_test.go internal/api/router.go
git commit -m "$(cat <<'EOF'
feat(api): GET /api/ledgers

Returns the active ledger name plus all registered ledgers sorted by
name. Each item carries name, path, and an active flag so the SPA
can render a switcher without a follow-up request.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 6: `handleActiveLedger` + `GET /api/ledgers/active`

**Files:**
- Modify: `internal/api/ledgers.go`
- Modify: `internal/api/ledgers_test.go`
- Modify: `internal/api/router.go`

- [ ] **Step 6.1: Write the failing tests.**

Append to `internal/api/ledgers_test.go`:

```go
func TestHandleActiveLedger_None(t *testing.T) {
    ts, _, _, _ := newTestServerWithLedger(t)

    status, body := getJSON(t, ts.URL+"/api/ledgers/active")
    if status != http.StatusNotFound {
        t.Fatalf("status: got %d, want 404; body=%s", status, body)
    }
    var got errorBody
    if err := json.Unmarshal(body, &got); err != nil {
        t.Fatalf("unmarshal: %v; body=%s", err, body)
    }
    if got.Error != "no_active_ledger" {
        t.Errorf("error: got %q, want %q", got.Error, "no_active_ledger")
    }
}

func TestHandleActiveLedger_Set(t *testing.T) {
    ts, reg, appDir, _ := newTestServerWithLedger(t)

    path := filepath.Join(appDir, "ledgers", "alpha.db")
    if err := reg.Add("alpha", path); err != nil {
        t.Fatalf("add: %v", err)
    }
    if err := reg.Switch("alpha"); err != nil {
        t.Fatalf("switch: %v", err)
    }

    status, body := getJSON(t, ts.URL+"/api/ledgers/active")
    if status != http.StatusOK {
        t.Fatalf("status: got %d, want 200; body=%s", status, body)
    }
    var got struct {
        Name   string `json:"name"`
        Path   string `json:"path"`
        Active bool   `json:"active"`
    }
    if err := json.Unmarshal(body, &got); err != nil {
        t.Fatalf("unmarshal: %v; body=%s", err, body)
    }
    if got.Name != "alpha" || got.Path != path || !got.Active {
        t.Errorf("body: got %+v, want {alpha, %s, true}", got, path)
    }
}
```

- [ ] **Step 6.2: Run the tests to verify they fail.**

Run: `go test ./internal/api/ -run TestHandleActiveLedger -v`
Expected: 404 (route not registered) for both.

- [ ] **Step 6.3: Add the handler.**

Append to `internal/api/ledgers.go`:

```go
func (s *Server) handleActiveLedger(w http.ResponseWriter, r *http.Request) error {
    name := s.registry.ActiveName()
    if name == "" {
        return ledger.ErrNoActiveLedger
    }
    e, ok := s.registry.EntryFor(name)
    if !ok {
        return fmt.Errorf("%w: %q", ledger.ErrLedgerNotFound, name)
    }
    return writeJSON(w, http.StatusOK, ledgerInfo{Name: name, Path: e.Path, Active: true})
}
```

Update the import block in `internal/api/ledgers.go`:

```go
import (
    "fmt"
    "net/http"

    "github.com/hance08/kea/internal/ledger"
)
```

- [ ] **Step 6.4: Register the route in `internal/api/router.go`.**

Add before the `/ledgers` line so chi resolves the static segment first (chi actually does this automatically, but keeping static-before-parametric in source order is the convention in this file). Insert:

```go
r.Method(http.MethodGet, "/ledgers/active", apiHandler(s.handleActiveLedger))
r.Method(http.MethodGet, "/ledgers", apiHandler(s.handleListLedgers))
```

(Replace the single `/ledgers` line from Task 5 with this pair.)

- [ ] **Step 6.5: Run the tests to verify they pass.**

Run: `go test ./internal/api/ -run TestHandleActiveLedger -v`
Expected: PASS for both.

- [ ] **Step 6.6: Commit.**

```bash
git add internal/api/ledgers.go internal/api/ledgers_test.go internal/api/router.go
git commit -m "$(cat <<'EOF'
feat(api): GET /api/ledgers/active

Returns the currently active ledger or 404 no_active_ledger when none
is set. Lets the SPA show the active ledger without parsing the full
list response.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 7: `handleCreateLedger` + `POST /api/ledgers`

**Files:**
- Modify: `internal/api/ledgers.go`
- Modify: `internal/api/ledgers_test.go`
- Modify: `internal/api/router.go`

- [ ] **Step 7.1: Write the failing tests.**

First, extend the existing import block at the top of `internal/api/ledgers_test.go` to include `os`:

```go
import (
    "bytes"
    "encoding/json"
    "io"
    "net/http"
    "os"
    "path/filepath"
    "testing"
)
```

Then append to the same file:

```go
func TestHandleCreateLedger_Success(t *testing.T) {
    ts, reg, appDir, _ := newTestServerWithLedger(t)

    status, body := postJSON(t, ts.URL+"/api/ledgers", map[string]any{"name": "alpha"})
    if status != http.StatusCreated {
        t.Fatalf("status: got %d, want 201; body=%s", status, body)
    }

    var got struct {
        Name   string `json:"name"`
        Path   string `json:"path"`
        Active bool   `json:"active"`
    }
    if err := json.Unmarshal(body, &got); err != nil {
        t.Fatalf("unmarshal: %v; body=%s", err, body)
    }
    wantPath := filepath.Join(appDir, "ledgers", "alpha.db")
    if got.Name != "alpha" || got.Path != wantPath || got.Active {
        t.Errorf("body: got %+v, want {alpha, %s, false}", got, wantPath)
    }
    if _, err := os.Stat(wantPath); err != nil {
        t.Errorf("db file not created at %q: %v", wantPath, err)
    }
    if _, ok := reg.EntryFor("alpha"); !ok {
        t.Errorf("registry missing alpha after create")
    }
}

func TestHandleCreateLedger_DuplicateName(t *testing.T) {
    ts, reg, appDir, _ := newTestServerWithLedger(t)

    if err := reg.Add("alpha", filepath.Join(appDir, "ledgers", "alpha.db")); err != nil {
        t.Fatalf("seed: %v", err)
    }

    status, body := postJSON(t, ts.URL+"/api/ledgers", map[string]any{"name": "alpha"})
    if status != http.StatusConflict {
        t.Fatalf("status: got %d, want 409; body=%s", status, body)
    }
    var eb errorBody
    if err := json.Unmarshal(body, &eb); err != nil {
        t.Fatalf("unmarshal: %v", err)
    }
    if eb.Error != "already_exists" {
        t.Errorf("error: got %q, want %q", eb.Error, "already_exists")
    }
}

func TestHandleCreateLedger_InvalidName(t *testing.T) {
    ts, _, _, _ := newTestServerWithLedger(t)

    cases := []struct {
        name    string
        payload map[string]any
    }{
        {"empty", map[string]any{"name": ""}},
        {"slash", map[string]any{"name": "a/b"}},
        {"backslash", map[string]any{"name": `a\b`}},
        {"dotdot", map[string]any{"name": "..foo"}},
        {"control_char", map[string]any{"name": "a\x00b"}},
    }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            status, body := postJSON(t, ts.URL+"/api/ledgers", tc.payload)
            if status != http.StatusBadRequest {
                t.Fatalf("status: got %d, want 400; body=%s", status, body)
            }
            var eb errorBody
            if err := json.Unmarshal(body, &eb); err != nil {
                t.Fatalf("unmarshal: %v", err)
            }
            if eb.Error != "validation_failed" || eb.Field != "name" {
                t.Errorf("error: got %+v, want validation_failed/name", eb)
            }
        })
    }
}

func TestHandleCreateLedger_UnknownField(t *testing.T) {
    ts, _, _, _ := newTestServerWithLedger(t)

    status, body := postJSON(t, ts.URL+"/api/ledgers", map[string]any{
        "name": "alpha",
        "path": "/tmp/evil.db",
    })
    if status != http.StatusBadRequest {
        t.Fatalf("status: got %d, want 400; body=%s", status, body)
    }
    var eb errorBody
    if err := json.Unmarshal(body, &eb); err != nil {
        t.Fatalf("unmarshal: %v", err)
    }
    if eb.Error != "validation_failed" {
        t.Errorf("error: got %q, want validation_failed", eb.Error)
    }
}
```

- [ ] **Step 7.2: Run the tests to verify they fail.**

Run: `go test ./internal/api/ -run TestHandleCreateLedger -v`
Expected: 404 (route missing) on all cases.

- [ ] **Step 7.3: Add `validateLedgerName` and the handler.**

Update the import block in `internal/api/ledgers.go` to include `os`, `path/filepath`, `strings`, `github.com/hance08/kea/internal/app`, and `github.com/hance08/kea/internal/service`:

```go
import (
    "fmt"
    "net/http"
    "os"
    "path/filepath"
    "strings"

    "github.com/hance08/kea/internal/app"
    "github.com/hance08/kea/internal/ledger"
    "github.com/hance08/kea/internal/service"
)
```

Append:

```go
func validateLedgerName(name string) error {
    if name == "" {
        return &service.ValidationError{Field: "name", Message: "name is required"}
    }
    if strings.ContainsAny(name, `/\`) || strings.Contains(name, "..") {
        return &service.ValidationError{Field: "name", Message: "name must not contain path separators"}
    }
    for _, r := range name {
        if r < 0x20 || r == 0x7f {
            return &service.ValidationError{Field: "name", Message: "name must not contain control characters"}
        }
    }
    return nil
}

func (s *Server) handleCreateLedger(w http.ResponseWriter, r *http.Request) error {
    var req createLedgerRequest
    if err := decodeJSON(r, &req); err != nil {
        return err
    }
    if err := validateLedgerName(req.Name); err != nil {
        return err
    }
    if _, exists := s.registry.EntryFor(req.Name); exists {
        return fmt.Errorf("%w: %q", ledger.ErrLedgerExists, req.Name)
    }
    dbPath := filepath.Join(s.appDir, "ledgers", req.Name+".db")
    if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
        return fmt.Errorf("create ledger directory: %w", err)
    }
    if err := app.InitLedgerDB(dbPath, s.migrations); err != nil {
        return fmt.Errorf("initialize database: %w", err)
    }
    if err := s.registry.Add(req.Name, dbPath); err != nil {
        return err
    }
    return writeJSON(w, http.StatusCreated, ledgerInfo{Name: req.Name, Path: dbPath, Active: false})
}
```

- [ ] **Step 7.4: Register the route in `internal/api/router.go`.**

Inside the `r.Route("/api", ...)` block, alongside the existing ledger routes:

```go
r.Method(http.MethodPost, "/ledgers", apiHandler(s.handleCreateLedger))
```

- [ ] **Step 7.5: Run the tests to verify they pass.**

Run: `go test ./internal/api/ -run TestHandleCreateLedger -v`
Expected: PASS across all subtests.

- [ ] **Step 7.6: Commit.**

```bash
git add internal/api/ledgers.go internal/api/ledgers_test.go internal/api/router.go
git commit -m "$(cat <<'EOF'
feat(api): POST /api/ledgers

Creates a new ledger at <appDir>/ledgers/<name>.db, registers it, and
returns the new LedgerInfo. validateLedgerName rejects empty names,
path separators, .., and ASCII control characters.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 8: `handleSwitchLedger` + `POST /api/ledgers/switch`

**Files:**
- Modify: `internal/api/ledgers.go`
- Modify: `internal/api/ledgers_test.go`
- Modify: `internal/api/router.go`

- [ ] **Step 8.1: Write the failing tests.**

Append to `internal/api/ledgers_test.go`:

```go
func TestHandleSwitchLedger_Success(t *testing.T) {
    ts, reg, appDir, calls := newTestServerWithLedger(t)

    pathA := filepath.Join(appDir, "ledgers", "alpha.db")
    pathB := filepath.Join(appDir, "ledgers", "beta.db")
    if err := reg.Add("alpha", pathA); err != nil {
        t.Fatalf("add alpha: %v", err)
    }
    if err := reg.Add("beta", pathB); err != nil {
        t.Fatalf("add beta: %v", err)
    }
    if err := reg.Switch("alpha"); err != nil {
        t.Fatalf("seed switch: %v", err)
    }

    status, body := postJSON(t, ts.URL+"/api/ledgers/switch", map[string]any{"name": "beta"})
    if status != http.StatusOK {
        t.Fatalf("status: got %d, want 200; body=%s", status, body)
    }

    var got struct {
        Name   string `json:"name"`
        Path   string `json:"path"`
        Active bool   `json:"active"`
    }
    if err := json.Unmarshal(body, &got); err != nil {
        t.Fatalf("unmarshal: %v; body=%s", err, body)
    }
    if got.Name != "beta" || got.Path != pathB || !got.Active {
        t.Errorf("body: got %+v, want {beta, %s, true}", got, pathB)
    }
    if reg.ActiveName() != "beta" {
        t.Errorf("registry active: got %q, want beta", reg.ActiveName())
    }
    if len(*calls) != 1 || (*calls)[0] != "beta" {
        t.Errorf("switchFn calls: got %v, want [beta]", *calls)
    }
}

func TestHandleSwitchLedger_Unknown(t *testing.T) {
    ts, _, _, _ := newTestServerWithLedger(t)

    status, body := postJSON(t, ts.URL+"/api/ledgers/switch", map[string]any{"name": "ghost"})
    if status != http.StatusNotFound {
        t.Fatalf("status: got %d, want 404; body=%s", status, body)
    }
    var eb errorBody
    if err := json.Unmarshal(body, &eb); err != nil {
        t.Fatalf("unmarshal: %v", err)
    }
    if eb.Error != "not_found" {
        t.Errorf("error: got %q, want not_found", eb.Error)
    }
}

func TestHandleSwitchLedger_EmptyName(t *testing.T) {
    ts, _, _, _ := newTestServerWithLedger(t)

    status, body := postJSON(t, ts.URL+"/api/ledgers/switch", map[string]any{"name": ""})
    if status != http.StatusBadRequest {
        t.Fatalf("status: got %d, want 400; body=%s", status, body)
    }
    var eb errorBody
    if err := json.Unmarshal(body, &eb); err != nil {
        t.Fatalf("unmarshal: %v", err)
    }
    if eb.Error != "validation_failed" || eb.Field != "name" {
        t.Errorf("error: got %+v, want validation_failed/name", eb)
    }
}
```

- [ ] **Step 8.2: Run the tests to verify they fail.**

Run: `go test ./internal/api/ -run TestHandleSwitchLedger -v`
Expected: 404 (route missing).

- [ ] **Step 8.3: Add the handler.**

Append to `internal/api/ledgers.go`:

```go
func (s *Server) handleSwitchLedger(w http.ResponseWriter, r *http.Request) error {
    var req switchLedgerRequest
    if err := decodeJSON(r, &req); err != nil {
        return err
    }
    if err := validateLedgerName(req.Name); err != nil {
        return err
    }
    if err := s.switchLedger(req.Name); err != nil {
        return err
    }
    e, _ := s.registry.EntryFor(req.Name)
    return writeJSON(w, http.StatusOK, ledgerInfo{Name: req.Name, Path: e.Path, Active: true})
}
```

- [ ] **Step 8.4: Register the route.**

Inside the `r.Route("/api", ...)` block:

```go
r.Method(http.MethodPost, "/ledgers/switch", apiHandler(s.handleSwitchLedger))
```

- [ ] **Step 8.5: Run the tests to verify they pass.**

Run: `go test ./internal/api/ -run TestHandleSwitchLedger -v`
Expected: PASS.

- [ ] **Step 8.6: Commit.**

```bash
git add internal/api/ledgers.go internal/api/ledgers_test.go internal/api/router.go
git commit -m "$(cat <<'EOF'
feat(api): POST /api/ledgers/switch

Synchronously swaps the active ledger via the injected switchFn so
the response only resolves after the new store is live. Returns the
now-active LedgerInfo or routes ErrLedgerNotFound to 404.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 9: `handleDeleteLedger` + `DELETE /api/ledgers/{name}`

**Files:**
- Modify: `internal/api/ledgers.go`
- Modify: `internal/api/ledgers_test.go`
- Modify: `internal/api/router.go`

- [ ] **Step 9.1: Write the failing tests.**

Append to `internal/api/ledgers_test.go`:

```go
func TestHandleDeleteLedger_Inactive(t *testing.T) {
    ts, reg, appDir, _ := newTestServerWithLedger(t)

    pathA := filepath.Join(appDir, "ledgers", "alpha.db")
    pathB := filepath.Join(appDir, "ledgers", "beta.db")
    if err := reg.Add("alpha", pathA); err != nil {
        t.Fatalf("add alpha: %v", err)
    }
    if err := reg.Add("beta", pathB); err != nil {
        t.Fatalf("add beta: %v", err)
    }
    if err := reg.Switch("alpha"); err != nil {
        t.Fatalf("switch: %v", err)
    }

    // Create the beta file on disk so we can assert the file is untouched
    // after DELETE.
    if err := os.MkdirAll(filepath.Dir(pathB), 0755); err != nil {
        t.Fatalf("mkdir: %v", err)
    }
    if err := os.WriteFile(pathB, []byte("sqlite-magic"), 0644); err != nil {
        t.Fatalf("write beta: %v", err)
    }

    status, body := deleteURL(t, ts.URL+"/api/ledgers/beta")
    if status != http.StatusOK {
        t.Fatalf("status: got %d, want 200; body=%s", status, body)
    }
    var got struct {
        Deleted bool   `json:"deleted"`
        Name    string `json:"name"`
    }
    if err := json.Unmarshal(body, &got); err != nil {
        t.Fatalf("unmarshal: %v", err)
    }
    if !got.Deleted || got.Name != "beta" {
        t.Errorf("body: got %+v, want {deleted:true, name:beta}", got)
    }
    if _, ok := reg.EntryFor("beta"); ok {
        t.Errorf("registry still has beta after delete")
    }
    if _, err := os.Stat(pathB); err != nil {
        t.Errorf("beta db file removed despite unregister-only policy: %v", err)
    }
}

func TestHandleDeleteLedger_Active(t *testing.T) {
    ts, reg, appDir, _ := newTestServerWithLedger(t)

    if err := reg.Add("alpha", filepath.Join(appDir, "ledgers", "alpha.db")); err != nil {
        t.Fatalf("add: %v", err)
    }
    if err := reg.Switch("alpha"); err != nil {
        t.Fatalf("switch: %v", err)
    }

    status, body := deleteURL(t, ts.URL+"/api/ledgers/alpha")
    if status != http.StatusConflict {
        t.Fatalf("status: got %d, want 409; body=%s", status, body)
    }
    var eb errorBody
    if err := json.Unmarshal(body, &eb); err != nil {
        t.Fatalf("unmarshal: %v", err)
    }
    if eb.Error != "cannot_remove_active" {
        t.Errorf("error: got %q, want cannot_remove_active", eb.Error)
    }
}

func TestHandleDeleteLedger_Unknown(t *testing.T) {
    ts, _, _, _ := newTestServerWithLedger(t)

    status, body := deleteURL(t, ts.URL+"/api/ledgers/ghost")
    if status != http.StatusNotFound {
        t.Fatalf("status: got %d, want 404; body=%s", status, body)
    }
    var eb errorBody
    if err := json.Unmarshal(body, &eb); err != nil {
        t.Fatalf("unmarshal: %v", err)
    }
    if eb.Error != "not_found" {
        t.Errorf("error: got %q, want not_found", eb.Error)
    }
}
```

- [ ] **Step 9.2: Run the tests to verify they fail.**

Run: `go test ./internal/api/ -run TestHandleDeleteLedger -v`
Expected: 404 (route missing) on the inactive case; the active case may already return 404 too.

- [ ] **Step 9.3: Add the handler.**

Append to `internal/api/ledgers.go`. Add `github.com/go-chi/chi/v5` to imports:

```go
import (
    "fmt"
    "net/http"
    "os"
    "path/filepath"
    "strings"

    "github.com/go-chi/chi/v5"

    "github.com/hance08/kea/internal/app"
    "github.com/hance08/kea/internal/ledger"
    "github.com/hance08/kea/internal/service"
)
```

Handler:

```go
func (s *Server) handleDeleteLedger(w http.ResponseWriter, r *http.Request) error {
    name := chi.URLParam(r, "name")
    if err := s.registry.Remove(name, false); err != nil {
        return err
    }
    return writeJSON(w, http.StatusOK, map[string]any{"deleted": true, "name": name})
}
```

- [ ] **Step 9.4: Register the route.**

Inside the `r.Route("/api", ...)` block:

```go
r.Method(http.MethodDelete, "/ledgers/{name}", apiHandler(s.handleDeleteLedger))
```

- [ ] **Step 9.5: Run the tests to verify they pass.**

Run: `go test ./internal/api/ -run TestHandleDeleteLedger -v`
Expected: PASS for all three subtests.

- [ ] **Step 9.6: Commit.**

```bash
git add internal/api/ledgers.go internal/api/ledgers_test.go internal/api/router.go
git commit -m "$(cat <<'EOF'
feat(api): DELETE /api/ledgers/{name}

Unregisters a ledger (DB file stays on disk). The active ledger
cannot be removed and routes ErrRemoveActive to 409
cannot_remove_active.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 10: Final verification

- [ ] **Step 10.1: Run the full test suite.**

Run: `go test ./...`
Expected: PASS across every package, no skips beyond what was skipped before this work.

- [ ] **Step 10.2: Run the build to confirm a clean binary.**

Run: `go build ./...`
Expected: no output.

- [ ] **Step 10.3: Smoke-test the routes locally (optional but recommended).**

This step exercises the actual `kea serve` binary against a temp `KEA_DATA_DIR`-style setup. Skip if the test suite has already covered everything.

```bash
# In one terminal:
KEA_LEDGER= go run ./cmd/kea serve

# In another:
curl -s http://localhost:8080/api/ledgers | jq .
curl -s -X POST http://localhost:8080/api/ledgers \
    -H 'Content-Type: application/json' -d '{"name": "smoketest"}' | jq .
curl -s -X POST http://localhost:8080/api/ledgers/switch \
    -H 'Content-Type: application/json' -d '{"name": "smoketest"}' | jq .
curl -s http://localhost:8080/api/ledgers/active | jq .
```

Expected: each request returns the documented JSON shape. The `smoketest` ledger appears in the registry and ledgers.yaml updates after switch. Stop the server with Ctrl+C; clean up the smoketest ledger via `kea ledger remove smoketest`.

- [ ] **Step 10.4: Update CLAUDE.md.**

Open `CLAUDE.md`. If the Architecture section lists endpoints or recent specs, append one line under the appropriate section pointing at `docs/superpowers/specs/2026-06-04-web-api-ledgers-design.md`. If the file doesn't track endpoints (current state), skip this step.

- [ ] **Step 10.5: Open a PR.**

Push the branch and open a PR with the title `feat(api): ledger endpoints` and a body that links to the spec and lists the five endpoints + the four newly mapped sentinels.

---

## Spec Coverage Self-Review

Cross-check against `docs/superpowers/specs/2026-06-04-web-api-ledgers-design.md`:

| Spec requirement | Implemented in |
|------------------|----------------|
| `GET /api/ledgers` returns `{active, items[]}` sorted by name with active flag | Task 5 |
| `GET /api/ledgers/active` returns LedgerInfo or 404 no_active_ledger | Task 6 |
| `POST /api/ledgers` creates at `<appDir>/ledgers/<name>.db`, returns 201 | Task 7 |
| `POST /api/ledgers/switch` synchronous swap, returns 200 LedgerInfo | Task 8 (handler) + Task 1 (App.SwitchLedger) |
| `DELETE /api/ledgers/{name}` unregister-only, returns `{deleted, name}` | Task 9 |
| `LedgerInfo` DTO shape `{name, path, active}` | Task 5 |
| Error mapping for `ErrLedgerNotFound`/`ErrLedgerExists`/`ErrRemoveActive`/`ErrNoActiveLedger` | Task 3 |
| `validateLedgerName` rejects empty/`/`/`\`/`..`/control chars | Task 7 |
| `decodeJSON` rejects unknown fields | Task 7 (covered by `TestHandleCreateLedger_UnknownField`) |
| `App.SwitchLedger` swap-then-YAML ordering, idempotent watcher reload | Task 1 |
| `Server` struct gains `registry`, `migrations`, `appDir`, `switchFn` | Task 2 |
| `cmd/serve.go` wires `application.SwitchLedger` | Task 2 |
| `newTestServerWithLedger` helper with fake switchFn | Task 4 |
| `internal/app/app_test.go` covers real swap behavior | Task 1 |
| `errors_test.go` rows for each new sentinel + wrapped-with-`%w` | Task 3 |
| Route registration order: static segments before parametric | Task 6 (re-order) + Task 9 |
| All four sentinels become exercisable from API tests | Tasks 6, 7, 8, 9 |

No gaps detected. No "TODO/TBD" in the plan body. Type names are consistent: `ledgerInfo`, `ledgerListResponse`, `createLedgerRequest`, `switchLedgerRequest`, `validateLedgerName`, `App.SwitchLedger`, `Server.switchFn`.
