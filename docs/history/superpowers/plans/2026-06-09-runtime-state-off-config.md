# Runtime State off `*config.Config` Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move the two runtime-mutable fields (`ActiveLedger`, `Database.Path`) off `*config.Config` and onto `*App` behind a `sync.RWMutex`, exposed via a value-type `RuntimeState` snapshot accessor. Eliminates the data race triggered by `TestApp_WatchSwapsStoreOnExternalSwitch` and removes the redundant cfg cache.

**Architecture:**
`*config.Config` becomes pure loaded settings. `*App` gains `runtime RuntimeState` guarded by `runtimeMu sync.RWMutex`, with `RuntimeState() RuntimeState` for callers and an internal `setRuntime(s)` for the two write sites (the `OnSwitch` callback and `SwitchLedger`). `NewApp` seeds the initial state from `Registry.Active()` / `Registry.ActiveName()` instead of reading the pre-mutated `cfg.Database.Path`.

**Tech Stack:** Go 1.x, stdlib `sync`, `testify` (existing tests), `httptest`, the existing `migrations.FS`/`store`/`ledger` packages.

**Spec:** `docs/superpowers/specs/2026-06-09-runtime-state-off-config-design.md`

---

## File Structure

**Created:**
- `cmd/info_test.go` — new test file; first test for the `info` command. Holds a fake `InfoProvider` and `SystemInfoView`.

**Modified:**
- `internal/config/config.go` — remove `ActiveLedger` field.
- `internal/app/app.go` — add `RuntimeState` type, `runtimeMu`/`runtime` fields, `RuntimeState()`, `setRuntime()`; rewire `NewApp` to seed from registry; rewire `OnSwitch` callback and `SwitchLedger` to use `setRuntime`; delete the unreachable empty-path fallback.
- `internal/app/app_test.go` — drop `cfg.ActiveLedger = "a"` lines; switch all assertions from `a.cfg.*` / `a.Config().ActiveLedger` / `a.Config().Database.Path` to `a.RuntimeState()`; add three new tests.
- `cmd/info.go` — widen `InfoProvider` interface; change `NewInfoCmd` signature to take `*app.App`; read from `RuntimeState()` instead of `Config()`.
- `cmd/root.go` — delete the two cfg write lines at 122-123; change `NewInfoCmd(application.Service)` to `NewInfoCmd(application)`.

**Untouched (in scope but no change needed):** `internal/api/*`, `internal/service/*`, `internal/ledger/*`, the `~`-expansion at `cmd/root.go:317-322`.

---

## Task 1: Add `RuntimeState` type and unguarded accessor (red)

Establishes the public API surface. Test-first: write a test that calls a method that doesn't exist yet.

**Files:**
- Modify: `internal/app/app.go`
- Test: `internal/app/app_test.go`

- [ ] **Step 1: Add the failing test**

Append to `internal/app/app_test.go`:

```go
func TestApp_RuntimeStateInitialFromRegistry(t *testing.T) {
	a, _, pathA, _ := newTestApp(t)

	got := a.RuntimeState()
	want := RuntimeState{ActiveLedger: "a", DatabasePath: pathA}
	if got != want {
		t.Errorf("RuntimeState: got %+v, want %+v", got, want)
	}
}
```

- [ ] **Step 2: Run to confirm failure**

```bash
go test ./internal/app/ -run TestApp_RuntimeStateInitialFromRegistry
```

Expected: compile error — `a.RuntimeState undefined` and `RuntimeState undefined`.

- [ ] **Step 3: Add the type, field, and accessor**

In `internal/app/app.go`, add to the imports if missing:

```go
"sync"
```

Replace the `App` struct (currently lines 19-25):

```go
type App struct {
	Service    *service.Service
	Registry   *ledger.Registry
	store      *store.Store
	migrations fs.FS
	cfg        *config.Config

	runtimeMu sync.RWMutex
	runtime   RuntimeState
}

// RuntimeState is a value-type snapshot of the active ledger name and the
// database path the store is currently open against. Returned by value from
// App.RuntimeState so callers get a consistent pair under one lock.
type RuntimeState struct {
	ActiveLedger string
	DatabasePath string
}

// RuntimeState returns a snapshot of the currently-active ledger name and
// the database path the store is open against. Safe for concurrent callers.
func (a *App) RuntimeState() RuntimeState {
	a.runtimeMu.RLock()
	defer a.runtimeMu.RUnlock()
	return a.runtime
}

// setRuntime atomically replaces the runtime state. Internal — called from
// NewApp's OnSwitch callback and from SwitchLedger.
func (a *App) setRuntime(s RuntimeState) {
	a.runtimeMu.Lock()
	a.runtime = s
	a.runtimeMu.Unlock()
}
```

Also update `newTestApp` in `internal/app/app_test.go` to seed the runtime state on the returned App (the helper bypasses `NewApp`). Locate the `return &App{...}` block at lines 64-69 and change to:

```go
	return &App{
		Registry:   reg,
		store:      st,
		migrations: migrations.FS,
		cfg:        cfg,
		runtime:    RuntimeState{ActiveLedger: "a", DatabasePath: pathA},
	}, tempDir, pathA, pathB
```

- [ ] **Step 4: Run to confirm pass**

```bash
go test ./internal/app/ -run TestApp_RuntimeStateInitialFromRegistry
```

Expected: PASS.

- [ ] **Step 5: Confirm the existing suite still compiles**

```bash
go build ./...
```

Expected: build OK. The other `internal/app/app_test.go` tests still read `a.cfg.*` and `a.Config().*`; those still compile because `cfg.ActiveLedger` and `cfg.Database.Path` still exist. We migrate them in later tasks.

- [ ] **Step 6: Commit**

```bash
git add internal/app/app.go internal/app/app_test.go
git commit -m "feat(app): add RuntimeState type and snapshot accessor"
```

---

## Task 2: Wire `setRuntime` into `OnSwitch` and `SwitchLedger`

Both write paths now update the new `App.runtime` field. We keep the cfg writes for one more task so existing assertions keep working — they get removed in Task 3.

**Files:**
- Modify: `internal/app/app.go`
- Test: `internal/app/app_test.go`

- [ ] **Step 1: Add the failing test**

Append to `internal/app/app_test.go`:

```go
func TestApp_RuntimeStateAfterSwitchLedger(t *testing.T) {
	a, _, _, pathB := newTestApp(t)

	if err := a.SwitchLedger("b"); err != nil {
		t.Fatalf("SwitchLedger: %v", err)
	}

	got := a.RuntimeState()
	want := RuntimeState{ActiveLedger: "b", DatabasePath: pathB}
	if got != want {
		t.Errorf("RuntimeState after switch: got %+v, want %+v", got, want)
	}
}
```

- [ ] **Step 2: Run to confirm failure**

```bash
go test ./internal/app/ -run TestApp_RuntimeStateAfterSwitchLedger
```

Expected: FAIL — `RuntimeState` still holds `{"a", pathA}` because `SwitchLedger` does not yet call `setRuntime`.

- [ ] **Step 3: Update `OnSwitch` callback and `SwitchLedger` in `internal/app/app.go`**

Locate the `OnSwitch` callback (currently lines 50-57). Replace with:

```go
	app := &App{
		Service:    svc,
		Registry:   registry,
		store:      dbStore,
		migrations: migrationFS,
		cfg:        cfg,
		runtime:    RuntimeState{ActiveLedger: cfg.ActiveLedger, DatabasePath: dbPathRaw},
	}

	registry.OnSwitch(func(name, path string) {
		if err := dbStore.Swap(path, migrationFS); err != nil {
			fmt.Fprintf(os.Stderr, "ledger switch failed: %v\n", err)
			return
		}
		app.setRuntime(RuntimeState{ActiveLedger: name, DatabasePath: path})
		cfg.ActiveLedger = name
		cfg.Database.Path = path
	})
```

(Note: we now construct `app` earlier — before the callback — so the closure can call `app.setRuntime`. The cfg writes stay for one more task; Task 3 removes them.)

Then locate the `return &App{...}` block (currently lines 66-72) at the bottom of `NewApp`. Replace the return with the already-constructed `app`:

```go
	cleanup := func() {
		registry.StopWatch()
		if err := dbStore.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Error closing DB: %v\n", err)
		}
	}

	return app, cleanup, nil
}
```

Locate `SwitchLedger` (currently lines 92-103). Replace with:

```go
func (a *App) SwitchLedger(name string) error {
	entry, ok := a.Registry.EntryFor(name)
	if !ok {
		return fmt.Errorf("%w: %q", ledger.ErrLedgerNotFound, name)
	}
	if err := a.store.Swap(entry.Path, a.migrations); err != nil {
		return fmt.Errorf("swap store: %w", err)
	}
	a.setRuntime(RuntimeState{ActiveLedger: name, DatabasePath: entry.Path})
	a.cfg.ActiveLedger = name
	a.cfg.Database.Path = entry.Path
	return a.Registry.Switch(name)
}
```

- [ ] **Step 4: Run to confirm pass**

```bash
go test ./internal/app/ -run TestApp_RuntimeStateAfterSwitchLedger
```

Expected: PASS.

- [ ] **Step 5: Run the full app package suite**

```bash
go test ./internal/app/
```

Expected: PASS — all existing tests still pass because `cfg.*` writes remain.

- [ ] **Step 6: Commit**

```bash
git add internal/app/app.go internal/app/app_test.go
git commit -m "feat(app): write runtime state from OnSwitch and SwitchLedger"
```

---

## Task 3: Migrate `app_test.go` assertions from `cfg` to `RuntimeState`

Switch every existing `a.cfg.*` / `a.Config().*` read in this file to `a.RuntimeState()`. This isolates Task 4 (removing the cfg writes) from test churn.

**Files:**
- Modify: `internal/app/app_test.go`

- [ ] **Step 1: Update `TestSwitchLedger_SwapsStoreAndUpdatesConfig`**

Replace the assertions at lines 79-84:

```go
	got := a.RuntimeState()
	if got.ActiveLedger != "b" {
		t.Errorf("RuntimeState.ActiveLedger: got %q, want %q", got.ActiveLedger, "b")
	}
	if got.DatabasePath != pathB {
		t.Errorf("RuntimeState.DatabasePath: got %q, want %q", got.DatabasePath, pathB)
	}
```

Also rename the test to drop the now-misleading "Config" word:

```go
func TestSwitchLedger_SwapsStoreAndUpdatesRuntimeState(t *testing.T) {
```

- [ ] **Step 2: Update `TestSwitchLedger_UnknownNameReturnsErrLedgerNotFound`**

Replace lines 107-112:

```go
	got := a.RuntimeState()
	if got.ActiveLedger != "a" {
		t.Errorf("RuntimeState.ActiveLedger leaked: got %q, want %q", got.ActiveLedger, "a")
	}
	if got.DatabasePath != pathA {
		t.Errorf("RuntimeState.DatabasePath leaked: got %q, want %q", got.DatabasePath, pathA)
	}
```

- [ ] **Step 3: Update `TestSwitchLedger_FailedSwapLeavesStateUnchanged`**

Replace lines 140-145 with the same `RuntimeState` pattern:

```go
	got := a.RuntimeState()
	if got.ActiveLedger != "a" {
		t.Errorf("RuntimeState.ActiveLedger leaked: got %q, want %q", got.ActiveLedger, "a")
	}
	if got.DatabasePath != pathA {
		t.Errorf("RuntimeState.DatabasePath leaked: got %q, want %q", got.DatabasePath, pathA)
	}
```

- [ ] **Step 4: Update `TestApp_WatchSwapsStoreOnExternalSwitch`**

Replace the poll-loop and final assertions (currently lines 222-237):

```go
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if a.RuntimeState().ActiveLedger == "b" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	got := a.RuntimeState()
	if got.ActiveLedger != "b" {
		t.Fatalf("RuntimeState.ActiveLedger: got %q, want %q (external switch did not propagate)",
			got.ActiveLedger, "b")
	}
	if got.DatabasePath != pathB {
		t.Errorf("RuntimeState.DatabasePath: got %q, want %q after external switch",
			got.DatabasePath, pathB)
	}
```

- [ ] **Step 5: Run the full suite**

```bash
go test ./internal/app/
```

Expected: PASS.

- [ ] **Step 6: Run with the race detector**

```bash
go test -race ./internal/app/ -run TestApp_WatchSwapsStoreOnExternalSwitch
```

Expected: still reports a race on the `cfg.ActiveLedger = name` write at the `OnSwitch` callback vs. the cfg field elsewhere — but the **test's own** poll-loop is now race-free because it reads `RuntimeState()` under a lock. The remaining cfg race goes away in Task 4 when the writes are deleted.

If the race detector still reports a race on the OnSwitch callback's `cfg.ActiveLedger = name` line at this point — that is expected and is the race we are about to remove.

- [ ] **Step 7: Commit**

```bash
git add internal/app/app_test.go
git commit -m "test(app): assert against RuntimeState instead of cfg fields"
```

---

## Task 4: Drop `cfg.ActiveLedger` field and its writes

Delete the field from `*config.Config`, delete the `cfg.ActiveLedger = ...` writes in `OnSwitch` and `SwitchLedger`, and delete the seed in `newTestApp`. Also delete the seed at `cmd/root.go:123`. The corresponding `cfg.Database.Path` writes at lines 56 (callback) and 101 (`SwitchLedger`) and `cmd/root.go:122` are removed in this same task — they were only needed while the cfg cache existed, and `NewApp` will be migrated to read the path from the registry in Task 5.

Wait — `NewApp` still reads `cfg.Database.Path` at line 29 to bootstrap. We do that swap in Task 5. To keep this task atomic, we keep the `cfg.Database.Path = activePath` line at `cmd/root.go:122` alive for one more task, and we keep the cfg.Database.Path writes inside the callback and `SwitchLedger`. Only `cfg.ActiveLedger` and its writes leave in this task.

**Files:**
- Modify: `internal/config/config.go`, `internal/app/app.go`, `internal/app/app_test.go`, `cmd/root.go`, `cmd/info.go` (the field read at line 70)

- [ ] **Step 1: Remove the field from `internal/config/config.go`**

Replace the `Config` struct:

```go
type Config struct {
	Database   DatabaseConfig `mapstructure:"database"`
	Defaults   DefaultsConfig `mapstructure:"defaults"`
	Server     ServerConfig   `mapstructure:"server"`
	ConfigPath string         `mapstructure:"-"`
}
```

- [ ] **Step 2: Remove all writes and the seed in `internal/app/app.go`**

In the `OnSwitch` callback (set up in Task 2), remove the `cfg.ActiveLedger = name` line. The callback now reads:

```go
	registry.OnSwitch(func(name, path string) {
		if err := dbStore.Swap(path, migrationFS); err != nil {
			fmt.Fprintf(os.Stderr, "ledger switch failed: %v\n", err)
			return
		}
		app.setRuntime(RuntimeState{ActiveLedger: name, DatabasePath: path})
		cfg.Database.Path = path
	})
```

In `SwitchLedger`, remove the `a.cfg.ActiveLedger = name` line. It now reads:

```go
func (a *App) SwitchLedger(name string) error {
	entry, ok := a.Registry.EntryFor(name)
	if !ok {
		return fmt.Errorf("%w: %q", ledger.ErrLedgerNotFound, name)
	}
	if err := a.store.Swap(entry.Path, a.migrations); err != nil {
		return fmt.Errorf("swap store: %w", err)
	}
	a.setRuntime(RuntimeState{ActiveLedger: name, DatabasePath: entry.Path})
	a.cfg.Database.Path = entry.Path
	return a.Registry.Switch(name)
}
```

Also update the in-line `app := &App{...}` construction earlier in `NewApp` to drop the `cfg.ActiveLedger` reference. The `runtime` seed becomes:

```go
		runtime:    RuntimeState{ActiveLedger: registry.ActiveName(), DatabasePath: dbPathRaw},
```

- [ ] **Step 3: Remove the seed in `newTestApp`**

In `internal/app/app_test.go`, delete the line `cfg.ActiveLedger = "a"` (currently line 56). It is no longer a valid field.

- [ ] **Step 4: Remove the seed in `TestApp_WatchSwapsStoreOnExternalSwitch`**

Delete the line `cfg.ActiveLedger = "a"` (currently line 186). Same reason.

- [ ] **Step 5: Remove the cfg write at `cmd/root.go:123`**

Locate:

```go
		cfg.Database.Path = activePath
		cfg.ActiveLedger = registry.ActiveName()
```

Delete only the `cfg.ActiveLedger = registry.ActiveName()` line for now. The `cfg.Database.Path = activePath` write stays until Task 5.

- [ ] **Step 6: Update the field read in `cmd/info.go`**

The `InfoProvider` interface and the runner both change: the interface gains `RuntimeState()`, and the runner reads the active ledger name and the DB path from `RuntimeState()` instead of `Config()`.

Replace the `InfoProvider` interface (currently lines 17-19):

```go
type InfoProvider interface {
	Config() *config.Config
	RuntimeState() app.RuntimeState
}
```

Replace `infoRunner.Run` (lines 50-81):

```go
func (r *infoRunner) Run() error {
	configPath := r.svc.Config().ConfigPath
	if configPath == "" {
		configPath = "(None, using defaults)"
	}

	rt := r.svc.RuntimeState()
	rawDBPath := rt.DatabasePath
	if rawDBPath == "" {
		appDir := getAppDataDirOrPanic()
		rawDBPath = filepath.Join(appDir, "kea.db")
	}
	expandedDBPath, _ := expandPath(rawDBPath)

	dbExists := false
	if _, err := os.Stat(expandedDBPath); err == nil {
		dbExists = true
	}

	info := views.SystemInfo{
		ConfigPath:      configPath,
		ActiveLedger:    rt.ActiveLedger,
		DBPath:          expandedDBPath,
		DBExists:        dbExists,
		DefaultCurrency: r.svc.Config().Defaults.Currency,
		AppDataDir:      getAppDataDirOrPanic(),
	}

	if r.json {
		return views.WriteJSON(views.ToJSONSystemInfo(info))
	}
	return r.view.Render(info)
}
```

The `NewInfoCmd` signature still takes `*service.Service` here — but `*service.Service` does not satisfy the widened `InfoProvider` interface (it has no `RuntimeState()` method). So `NewInfoCmd` must change in this task too. Update `NewInfoCmd` (lines 35-48):

```go
func NewInfoCmd(application *app.App) *cobra.Command {
	flags := &infoFlags{}
	cmd := &cobra.Command{
		Use:   "info",
		Short: "Display application information",
		Long:  `Display current configuration, database path, and system details.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			runner := &infoRunner{svc: application, view: views.NewSystemInfoView(), json: flags.JSON}
			return runner.Run()
		},
	}
	cmd.Flags().BoolVarP(&flags.JSON, "json", "j", false, "output as JSON")
	return cmd
}
```

Remove the now-unused `"github.com/hance08/kea/internal/service"` import if it's no longer referenced. Add `"github.com/hance08/kea/internal/app"` if not already imported (it is — line 10).

- [ ] **Step 7: Update the caller in `cmd/root.go`**

Locate line 148:

```go
		rootCmd.AddCommand(NewInfoCmd(application.Service))
```

Change to:

```go
		rootCmd.AddCommand(NewInfoCmd(application))
```

- [ ] **Step 8: Verify the tree compiles**

```bash
go build ./...
```

Expected: build OK.

- [ ] **Step 9: Run all tests**

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 10: Run with the race detector on the canary test**

```bash
go test -race ./internal/app/ -run TestApp_WatchSwapsStoreOnExternalSwitch
```

Expected: no race reports. The only remaining cfg field touched concurrently is `cfg.Database.Path`, which we delete the writes for in Task 5.

If a race is still reported on `cfg.Database.Path` at this point: that is expected — it goes away in Task 5.

- [ ] **Step 11: Commit**

```bash
git add internal/config/config.go internal/app/app.go internal/app/app_test.go cmd/root.go cmd/info.go
git commit -m "refactor(config): remove ActiveLedger field; readers use RuntimeState"
```

---

## Task 5: Drop runtime writes to `cfg.Database.Path`; have `NewApp` read from registry

`Database.Path` stays as a yaml-loaded settings field, but stops being mutated at runtime. `NewApp` reads the resolved active path from `Registry.Active()` directly.

**Files:**
- Modify: `internal/app/app.go`, `cmd/root.go`

- [ ] **Step 1: Rewrite the head of `NewApp` to read from the registry**

In `internal/app/app.go`, replace the head of `NewApp` (currently lines 28-48 — the section from `func NewApp` through `svc := service.NewService(...)`):

```go
func NewApp(cfg *config.Config, registry *ledger.Registry, migrationFS fs.FS) (*App, func(), error) {
	dbPath, err := registry.Active()
	if err != nil {
		return nil, nil, fmt.Errorf("resolve active ledger: %w", err)
	}

	if err := backup.Run(dbPath, nil); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: backup failed: %v\n", err)
	}

	dbStore, err := store.NewStore(dbPath, migrationFS)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	svc := service.NewService(dbStore, dbStore, dbStore, cfg)
```

Two things change here:
- The empty-path fallback (`appDir, _ := GetAppDataDir(); dbPathRaw = filepath.Join(appDir, "kea.db")`) is removed. It is unreachable: `registry.Active()` either returns a non-empty path or an error.
- The `cfg.Database.Path` read is gone.

The runtime seed (`runtime: RuntimeState{ActiveLedger: registry.ActiveName(), DatabasePath: dbPathRaw}`) becomes:

```go
		runtime:    RuntimeState{ActiveLedger: registry.ActiveName(), DatabasePath: dbPath},
```

- [ ] **Step 2: Remove `cfg.Database.Path = path` from the `OnSwitch` callback**

The callback becomes:

```go
	registry.OnSwitch(func(name, path string) {
		if err := dbStore.Swap(path, migrationFS); err != nil {
			fmt.Fprintf(os.Stderr, "ledger switch failed: %v\n", err)
			return
		}
		app.setRuntime(RuntimeState{ActiveLedger: name, DatabasePath: path})
	})
```

- [ ] **Step 3: Remove `a.cfg.Database.Path = entry.Path` from `SwitchLedger`**

`SwitchLedger` becomes:

```go
func (a *App) SwitchLedger(name string) error {
	entry, ok := a.Registry.EntryFor(name)
	if !ok {
		return fmt.Errorf("%w: %q", ledger.ErrLedgerNotFound, name)
	}
	if err := a.store.Swap(entry.Path, a.migrations); err != nil {
		return fmt.Errorf("swap store: %w", err)
	}
	a.setRuntime(RuntimeState{ActiveLedger: name, DatabasePath: entry.Path})
	return a.Registry.Switch(name)
}
```

- [ ] **Step 4: Remove `cfg.Database.Path = activePath` from `cmd/root.go`**

Locate (currently around line 122):

```go
		// Inject the resolved DB path so app.NewApp and kea info both see it.
		cfg.Database.Path = activePath
```

Delete both the comment and the assignment. The block now goes straight from `activePath, err := registry.Active()` / `if err != nil` to `application, cleanup, err := app.NewApp(cfg, registry, migrations)`.

Also clean up the now-unused `activePath` variable. Since `app.NewApp` calls `registry.Active()` itself, the outer `registry.Active()` call is just being used to detect the "no ledger configured" case. Replace it with `registry.ActiveName()` plus an emptiness check, which avoids the duplicate path resolution:

Locate (currently around lines 110-119):

```go
		activePath, err := registry.Active()
		if err != nil {
			// No active ledger — only ledger commands are useful.
			pterm.Warning.Println("No ledger configured. Run: kea ledger add <name>")
			if err := rootCmd.ExecuteContext(context.Background()); err != nil {
				pterm.Error.Println(capitalize(err.Error()))
				return 1
			}
			return 0
		}
```

Replace with:

```go
		if _, err := registry.Active(); err != nil {
			// No active ledger — only ledger commands are useful.
			pterm.Warning.Println("No ledger configured. Run: kea ledger add <name>")
			if err := rootCmd.ExecuteContext(context.Background()); err != nil {
				pterm.Error.Println(capitalize(err.Error()))
				return 1
			}
			return 0
		}
```

- [ ] **Step 5: Verify the tree compiles**

```bash
go build ./...
```

Expected: build OK.

- [ ] **Step 6: Run all tests**

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 7: Run the canary under `-race`**

```bash
go test -race ./internal/app/ -run TestApp_WatchSwapsStoreOnExternalSwitch
```

Expected: no race reports.

- [ ] **Step 8: Run the full suite under `-race`**

```bash
go test -race ./...
```

Expected: no race reports.

- [ ] **Step 9: Commit**

```bash
git add internal/app/app.go cmd/root.go
git commit -m "refactor(app): seed runtime state from registry; stop mutating cfg.Database.Path"
```

---

## Task 6: Add dedicated race test for `RuntimeState`

Pins the invariant: a tight reader loop colliding with the watcher must produce no race reports.

**Files:**
- Modify: `internal/app/app_test.go`

- [ ] **Step 1: Add the test**

Append to `internal/app/app_test.go`:

```go
// TestApp_RuntimeStateRace asserts that a reader hammering RuntimeState() while
// the watcher's OnSwitch callback fires produces no race reports under -race.
// Run with: go test -race ./internal/app/ -run TestApp_RuntimeStateRace
func TestApp_RuntimeStateRace(t *testing.T) {
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

	a, cleanup, err := NewApp(cfg, reg, migrations.FS)
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	t.Cleanup(cleanup)

	ctx, cancel := context.WithCancel(t.Context())
	watchDone := make(chan struct{})
	go func() {
		defer close(watchDone)
		_ = a.Registry.Watch(ctx)
	}()

	// Let fsnotify install its watch.
	time.Sleep(200 * time.Millisecond)

	// Reader: hammer RuntimeState until the test signals stop.
	stop := make(chan struct{})
	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		for {
			select {
			case <-stop:
				return
			default:
				_ = a.RuntimeState()
			}
		}
	}()

	// Writer: trigger an external switch that fires OnSwitch.
	extReg, err := ledger.Load(tempDir)
	if err != nil {
		t.Fatalf("load external registry: %v", err)
	}
	if err := extReg.Switch("b"); err != nil {
		t.Fatalf("external switch: %v", err)
	}

	// Wait for the callback to land.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if a.RuntimeState().ActiveLedger == "b" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	close(stop)
	<-readerDone

	if got := a.RuntimeState(); got.ActiveLedger != "b" || got.DatabasePath != pathB {
		t.Errorf("RuntimeState: got %+v, want {b %s}", got, pathB)
	}

	cancel()
	select {
	case <-watchDone:
	case <-time.After(2 * time.Second):
		t.Fatal("watcher goroutine did not exit after context cancel")
	}
}
```

- [ ] **Step 2: Run under `-race`**

```bash
go test -race ./internal/app/ -run TestApp_RuntimeStateRace
```

Expected: PASS, no race reports.

- [ ] **Step 3: Run under `-race` repeatedly to flush out flakes**

```bash
go test -race -count=5 ./internal/app/ -run TestApp_RuntimeStateRace
```

Expected: PASS on all 5 runs.

- [ ] **Step 4: Commit**

```bash
git add internal/app/app_test.go
git commit -m "test(app): pin race-free RuntimeState reads under watcher writes"
```

---

## Task 7: Add `cmd/info_test.go` covering the runtime-vs-config split

Pins that `kea info` reads the *runtime* path, not the *yaml-loaded* path. Prevents regression to the old conflation.

**Files:**
- Create: `cmd/info_test.go`

- [ ] **Step 1: Write the test file**

Create `cmd/info_test.go`:

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package cmd

import (
	"testing"

	"github.com/hance08/kea/internal/app"
	"github.com/hance08/kea/internal/config"
	"github.com/hance08/kea/ui/views"
)

type fakeInfoProvider struct {
	cfg     *config.Config
	runtime app.RuntimeState
}

func (f *fakeInfoProvider) Config() *config.Config       { return f.cfg }
func (f *fakeInfoProvider) RuntimeState() app.RuntimeState { return f.runtime }

type capturingView struct {
	got views.SystemInfo
}

func (c *capturingView) Render(info views.SystemInfo) error {
	c.got = info
	return nil
}

// TestInfoRunner_UsesRuntimeStateNotConfig asserts that the info command
// displays the path the store is actually open against (RuntimeState), not
// the value originally loaded from yaml (Config.Database.Path).
func TestInfoRunner_UsesRuntimeStateNotConfig(t *testing.T) {
	cfg := config.NewDefault()
	cfg.Database.Path = "/yaml/loaded/path.db"
	cfg.Defaults.Currency = "USD"

	provider := &fakeInfoProvider{
		cfg: cfg,
		runtime: app.RuntimeState{
			ActiveLedger: "alpha",
			DatabasePath: "/runtime/active/path.db",
		},
	}
	view := &capturingView{}

	runner := &infoRunner{svc: provider, view: view, json: false}
	if err := runner.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if view.got.ActiveLedger != "alpha" {
		t.Errorf("ActiveLedger: got %q, want %q", view.got.ActiveLedger, "alpha")
	}
	// expandPath leaves absolute paths untouched.
	if view.got.DBPath != "/runtime/active/path.db" {
		t.Errorf("DBPath: got %q, want %q (should come from RuntimeState, not Config)",
			view.got.DBPath, "/runtime/active/path.db")
	}
}

// TestInfoRunner_FallsBackToDefaultWhenRuntimePathEmpty pins the existing
// fallback: if RuntimeState.DatabasePath is empty, the runner falls back to
// <appDataDir>/kea.db. This branch is reachable in unit tests where a fake
// provider returns an empty runtime; production NewApp always seeds a
// non-empty path because registry.Active() guarantees one.
func TestInfoRunner_FallsBackToDefaultWhenRuntimePathEmpty(t *testing.T) {
	cfg := config.NewDefault()
	cfg.Defaults.Currency = "USD"

	provider := &fakeInfoProvider{
		cfg:     cfg,
		runtime: app.RuntimeState{},
	}
	view := &capturingView{}

	runner := &infoRunner{svc: provider, view: view, json: false}
	if err := runner.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if view.got.DBPath == "" {
		t.Errorf("DBPath fell through to empty; expected <appDataDir>/kea.db fallback")
	}
}
```

- [ ] **Step 2: Run the new tests**

```bash
go test ./cmd/ -run TestInfoRunner_
```

Expected: PASS.

- [ ] **Step 3: Run full suite**

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add cmd/info_test.go
git commit -m "test(cmd): info displays runtime path, not yaml-loaded path"
```

---

## Task 8: Final verification

End-to-end gate before declaring done.

- [ ] **Step 1: Full test suite**

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 2: Race-detector pass**

```bash
go test -race ./...
```

Expected: no race reports.

- [ ] **Step 3: Build**

```bash
go build ./...
```

Expected: OK.

- [ ] **Step 4: Sanity-grep for stragglers**

```bash
rg -n 'cfg\.ActiveLedger|Config\(\)\.ActiveLedger' --type go
```

Expected: no matches. (`cfg.ActiveLedger` should not exist anywhere — the field is removed.)

```bash
rg -n 'cfg\.Database\.Path = |cfg\.Database\.Path =[^=]' --type go
```

Expected: only the `~`-expansion at `cmd/root.go:322` and possibly the test fixture in `internal/app/app_test.go:55` (which seeds the yaml value, not a runtime mutation). No write inside any runtime-callable code path.

- [ ] **Step 5: Confirm the canary one more time**

```bash
go test -race -count=10 ./internal/app/ -run 'TestApp_RuntimeStateRace|TestApp_WatchSwapsStoreOnExternalSwitch'
```

Expected: 10/10 PASS, no race reports.

---

## Self-review notes

- **Spec coverage:**
  - `RuntimeState` type and accessor: Task 1.
  - `setRuntime` and write-site rewiring: Task 2.
  - Removal of `cfg.ActiveLedger` field: Task 4.
  - Removal of runtime writes to `cfg.Database.Path` and `NewApp` reading from registry: Task 5.
  - `cmd/root.go:122-123` deletion: Tasks 4 and 5.
  - `cmd/info.go` widening and signature change: Task 4.
  - `TestApp_WatchSwapsStoreOnExternalSwitch` poll-loop migration: Task 3.
  - "No leak on Swap failure" assertions migration: Task 3.
  - `TestApp_RuntimeStateInitialFromRegistry`: Task 1.
  - `TestApp_RuntimeStateAfterSwitchLedger`: Task 2.
  - `TestApp_RuntimeStateRace`: Task 6.
  - `TestInfoRunner_UsesRuntimeStateNotConfig`: Task 7.
  - Out-of-scope items (`~`-expansion at root.go:317-322, `/api/ledgers/*` handlers, registry API) are correctly untouched.
- **No placeholders.**
- **Type consistency:** `RuntimeState`, `runtimeMu`, `runtime`, `setRuntime`, `RuntimeState()` consistent across all tasks. `InfoProvider` interface widened in Task 4 and consumed identically in Task 7.
