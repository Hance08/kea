# Fix: Wire `Registry.Watch` into `kea serve`

**Status:** Design approved 2026-06-07.
**Closes:** Pre-development review §8 ("Known Limitations") item — "Ledger switch via CLI is invisible to running web server (#119)". This spec resolves that limitation.
**Sibling:** [`2026-06-05-registry-mutex-locking-design.md`](2026-06-05-registry-mutex-locking-design.md) — hardened the watcher's locking in anticipation of it being wired. This spec is the wiring.

## Problem

`Registry.Watch(ctx)` exists at [`internal/ledger/registry.go:269`](../../internal/ledger/registry.go) and is fully working: PR #181 hardened it with proper `r.mu` locking around state mutation, race tests passing under `-race`. The `OnSwitch` callback at [`internal/app/app.go:50`](../../internal/app/app.go) is wired to call `dbStore.Swap` (closing the old SQLite handle, opening the new one) and update `cfg.ActiveLedger` + `cfg.Database.Path`.

But [`cmd/serve.go::NewServeCmd::RunE`](../../cmd/serve.go) never calls `application.Registry.Watch(ctx)`. The watcher infrastructure is built and tested, but **never started in the server process**. A `grep -rn "\.Watch(" cmd/ internal/ --include="*.go"` outside test files returns zero hits.

Observed failure mode (reported by user in this session):
1. `kea serve` running on `:8080`.
2. SPA on `:5173` consuming `/api/balances`.
3. In a third terminal: `go run ./cmd/kea ledger switch <other-ledger>` — exits successfully, writes `ledgers.yaml`.
4. SPA still shows the old ledger's data. `curl /api/balances` still returns the old ledger's data.
5. The running server's `dbStore` keeps using the original SQLite handle.

This is exactly the split-brain behavior the pre-dev review §8 documented as a known limitation. The fix is one wiring change.

## Decision

Spawn `application.Registry.Watch(cmd.Context())` in a goroutine inside `serve`'s `RunE`, before starting the HTTP server. The watcher's lifetime is tied to the server's context — when SIGINT/SIGTERM cancels the context (already wired in `cmd/root.go`), the watcher's `select` loop exits via the `ctx.Done()` branch and returns `ctx.Err()`. The existing `NewApp` cleanup calls `Registry.StopWatch()` defensively for double-safety.

Three options considered:

1. **Goroutine in `serve.RunE`** *(chosen)*. ~6 lines. Serve-specific behavior in serve-specific code. The watcher is exactly what makes a long-running server reactive to ledger changes.
2. Start the watcher in `app.NewApp`. **Rejected**: every CLI command (`kea add`, `kea ledger list`, etc.) goes through `NewApp`. Each short-lived CLI invocation would spawn an unnecessary watcher goroutine, attempt to open fsnotify resources, and tear them down within milliseconds. Worse: the CLI's `Registry.Switch` call would trigger its own in-process watcher's `reload()` on its own write — pointless work and a self-race window.
3. Add an opt-in `--watch` CLI flag. YAGNI — for `kea serve`, the watcher should always run. There's no defensible "long-running server that ignores ledger switches" use case.

## Change

### `cmd/serve.go`

Current (verbatim, lines 18–32):

```go
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
```

Replace with:

```go
return &cobra.Command{
    Use:   "serve",
    Short: "Run the local web server",
    RunE: func(cmd *cobra.Command, args []string) error {
        logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))

        // Start the registry watcher so external `kea ledger switch`
        // calls made while the server is running cause this server to
        // swap stores. The watcher exits when ctx is cancelled; the
        // app's cleanup also calls StopWatch defensively.
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
}
```

New imports: `context` and `errors`. The other existing imports (`io/fs`, `log/slog`, `os`, `github.com/spf13/cobra`, `github.com/hance08/kea/internal/api`, `github.com/hance08/kea/internal/app`) are unchanged.

The `errors.Is(err, context.Canceled)` filter prevents the expected shutdown error (from `select`'s `<-ctx.Done()` branch returning `ctx.Err()`) from logging as a server error during graceful shutdown.

### Test

New `TestApp_WatchSwapsStoreOnExternalSwitch` in [`internal/app/app_test.go`](../../internal/app/app_test.go), alongside the existing `OnSwitch`-related tests. Exercises the full chain:

1. `t.TempDir()` for an isolated app directory.
2. Create two SQLite files (`a.db`, `b.db`) in that directory via `app.InitLedgerDB`.
3. Load a `Registry` via `ledger.Load(appDir)`, register both ledgers, set `a` active.
4. Build an `App` with `NewApp` using `a.db` as the initial active path.
5. Spawn `app.Registry.Watch(ctx)` in a goroutine — this is what `serve.RunE` will do.
6. `time.Sleep(200 * time.Millisecond)` — let fsnotify install its watch (same pattern as `TestWatch_FiresOnSwitch`).
7. From a **second** `Registry` instance (`ledger.Load(appDir)` again — simulating an external CLI process that loads the same file fresh), call `r2.Switch("b")`. This writes `ledgers.yaml`.
8. Wait up to 2 seconds for the OnSwitch callback to fire (poll on `a.Config().ActiveLedger == "b"`).
9. Assert `a.Config().ActiveLedger == "b"` and `a.Config().Database.Path == pathB`.
10. Cancel the context. Confirm the watcher goroutine exits (via a `done` channel pattern).

Uses real fsnotify and real SQLite files — no mocks. Mirrors the testing pattern of `internal/ledger/registry_test.go::TestWatch_FiresOnSwitch` but tests the App-level integration (callback wiring → store swap → cfg update) on top.

## Out of scope

- **The deferred `app.cfg` mutation race** (spawned-task chip). The OnSwitch callback writes `cfg.ActiveLedger` and `cfg.Database.Path`. Today, no API handler reads those concurrently (`/api/ledgers/active` reads from `Registry`, not `cfg`). This PR makes the writer goroutine actually run, but doesn't widen the read surface — the race remains theoretical. The chip stays.
- **Retry on `fsnotify.NewWatcher()` failure**. If the OS denies the watcher (inotify exhaustion, etc.), the error is logged once and the watcher goroutine exits. The server continues serving HTTP — only the CLI-switch-during-serve scenario silently breaks. Adequate for local single-user; a louder warning could come later if it becomes a real concern.
- **Removing the #119 note from the pre-dev review doc**. The doc is untracked (gitignored). Leaving as-is.
- **Suppressing the CLI process's own watcher**. The CLI never calls `Watch` — nothing to suppress.

## Verification

- `go test ./internal/app/... -run TestApp_WatchSwapsStoreOnExternalSwitch -v` — passes.
- `go test ./...` — green across all packages.
- `go test -race ./...` — green (existing race tests + new test all pass under `-race`).
- `go build ./...` — clean.
- Manual smoke (the reporter's exact scenario):
  1. `make run` in terminal 1.
  2. `make spa-dev` in terminal 2.
  3. Browse `http://localhost:5173/balances`, observe the active ledger's data.
  4. `go run ./cmd/kea ledger switch <other>` in terminal 3.
  5. Reload the SPA tab — Net Worth dashboard reflects the other ledger's accounts.
  6. `curl http://localhost:8080/api/balances` — returns the other ledger's data.

## Commit shape

Single commit, scope `fix(cmd)`:

```
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
```
