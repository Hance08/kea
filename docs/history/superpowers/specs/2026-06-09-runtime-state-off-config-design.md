# Move runtime-mutable state off `*config.Config` onto `*App`

Date: 2026-06-09
Status: Design — ready for implementation plan

## Problem

`internal/app/app.go`'s `OnSwitch` callback writes two fields on
`*config.Config` at runtime:

- `cfg.ActiveLedger` (string)
- `cfg.Database.Path` (string)

The callback fires from two goroutines:

1. The fsnotify watcher goroutine (`Registry.Watch`, wired up in
   `cmd/serve.go` by PR #186).
2. The HTTP request goroutine that handles
   `POST /api/ledgers/switch` (via `App.SwitchLedger`).

`go test -race ./internal/app/...` now reports a data race because
`TestApp_WatchSwapsStoreOnExternalSwitch` (added in PR #186) reads the
fields from its poll-loop while the watcher writes them. The race is
real per the race detector. It is not a production bug today — no API
handler reads `cfg.ActiveLedger` or `cfg.Database.Path`
concurrently — but it is a latent design smell:

- `*config.Config` semantically represents *loaded settings* (yaml +
  defaults). Two of its fields are *runtime state* that is mutated on
  user action.
- `Registry.ActiveName()` / `Registry.Active()` already return the
  same information under a mutex. The cfg fields are a redundant
  cache.

An earlier follow-up spec
(`2026-06-05-registry-mutex-locking-design.md`) explicitly deferred
this work: "runtime-mutable state probably belongs on `*App` not
`*config.Config`."

## Decision

Move the two runtime-mutable fields off `*config.Config` and onto
`*App`, guarded by a `sync.RWMutex` and exposed via a value-type
snapshot accessor `App.RuntimeState() RuntimeState`.

- `cfg.ActiveLedger` is removed entirely.
- `cfg.Database.Path` stays on Config but is no longer written at
  runtime. It represents the value loaded from yaml (used only by the
  startup `~`-expansion path and by the registry's legacy-DB
  migration).
- A new value-type `app.RuntimeState{ActiveLedger, DatabasePath}`
  carries the actually-in-use values. `*App` owns it, protected by
  `runtimeMu sync.RWMutex`.
- One read API: `App.RuntimeState()` returns a snapshot under the
  read lock. One internal write API: `App.setRuntime(s)` takes the
  write lock. Both call sites that previously mutated the cfg fields
  (`OnSwitch` callback and `App.SwitchLedger`) call `setRuntime`
  instead.

### Considered alternatives

- **Mutex on `*config.Config`.** Smallest diff, but adds a mutex to a
  struct that semantically should not need one. Leaves the
  conflation-of-settings-and-state smell in place. Rejected.
- **`atomic.Pointer[runtimeState]`.** Lock-free reads, but more
  ceremony than is justified — switches happen on user action, not on
  every request, so contention is irrelevant. Rejected for
  readability.
- **Drop the cache entirely and always read from
  `Registry.ActiveName()` / `Registry.Active()`.** Conceptually
  cleanest, but the resolved runtime path is what the *store* is open
  against, which is `App`'s concern, not the registry's. The registry
  knows what *should* be active; `App` knows what *is* active right
  now. They can diverge transiently during a swap. Rejected: keep
  `App` as the source of truth for "what the store is currently
  bound to."

## Architecture

```
*config.Config              pure loaded settings (yaml + defaults).
  Database.Path             value as loaded from yaml (used by
                            registry's legacy migration and the
                            startup ~-expansion).
                            No runtime mutation.
  (ActiveLedger removed)

*ledger.Registry            source of truth for "which ledger is
                            active right now". Already thread-safe.
                            Unchanged.

*app.App                    owns the resolved runtime state currently
                            in use by the store.
  runtimeMu sync.RWMutex
  runtime   RuntimeState
  RuntimeState()            public snapshot accessor
  setRuntime(s)             internal write
```

Writes from two goroutines (watcher, request) converge on
`setRuntime`. Readers (`cmd/info.go`, future API handlers) call
`RuntimeState()`.

Semantic split: `cfg.Database.Path` is *what the user wrote in yaml*
(immutable after load); `App.RuntimeState().DatabasePath` is *what the
store is actually open against right now* (mutable). The two are
equal at process start when no override happens; they diverge the
moment the user switches ledgers.

## API surface

### New types (`internal/app/`)

```go
// RuntimeState is a value-type snapshot of the active ledger and the
// database path the store is currently open against. Returned by
// value from App.RuntimeState so callers get a consistent pair under
// one lock.
type RuntimeState struct {
    ActiveLedger string
    DatabasePath string
}
```

### `*App` additions

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

func (a *App) RuntimeState() RuntimeState {
    a.runtimeMu.RLock()
    defer a.runtimeMu.RUnlock()
    return a.runtime
}

func (a *App) setRuntime(s RuntimeState) {
    a.runtimeMu.Lock()
    a.runtime = s
    a.runtimeMu.Unlock()
}
```

### `*config.Config` removal

```go
ActiveLedger string `mapstructure:"-"`   // deleted
```

### `InfoProvider` interface widens

```go
type InfoProvider interface {
    Config() *config.Config
    RuntimeState() app.RuntimeState
}
```

`NewInfoCmd` takes `*app.App` instead of `*service.Service`.

### `NewApp` behaviour

`NewApp` seeds its runtime state by calling `registry.Active()` and
`registry.ActiveName()` itself. It no longer reads
`cfg.Database.Path` as the source of the active path. Signature is
unchanged.

## Migration / change list

Ordered so the tree compiles at every step.

1. **`internal/config/config.go`** — remove the `ActiveLedger` field.
2. **`internal/app/app.go`**
   - Add the `RuntimeState` type, the `runtimeMu` / `runtime` fields,
     the `RuntimeState()` getter, and the internal `setRuntime`
     method.
   - In `NewApp`: drop the `dbPathRaw := cfg.Database.Path` read at
     line 29; call `registry.Active()` to resolve the path and
     `registry.ActiveName()` for the name (propagate any error from
     `registry.Active()` as `nil, nil, err`); seed `runtime` and pass
     the resolved path to `store.NewStore` and `backup.Run`. The
     pre-existing fallback at app.go:31-37 (defaulting to
     `<appDir>/kea.db` when path is empty) becomes unreachable because
     `registry.Active()` either returns a non-empty path or an error;
     delete it.
   - In the `OnSwitch` callback: replace the two `cfg.*` writes with
     `app.setRuntime(RuntimeState{name, path})`. The callback closes
     over `app` instead of `cfg`.
   - In `SwitchLedger`: replace the two `a.cfg.*` writes with
     `a.setRuntime(RuntimeState{name, entry.Path})`.
3. **`cmd/root.go`**
   - Delete lines 122-123 (`cfg.Database.Path = activePath` /
     `cfg.ActiveLedger = registry.ActiveName()`). `NewApp` reads
     from the registry directly.
   - Change `NewInfoCmd(application.Service)` to
     `NewInfoCmd(application)` at line 148.
   - Lines 317-322 (`~` expansion of yaml-loaded path) stay
     untouched.
4. **`cmd/info.go`**
   - Widen `InfoProvider` to include
     `RuntimeState() app.RuntimeState`.
   - Change `NewInfoCmd` signature from `*service.Service` to
     `*app.App`.
   - `infoRunner.Run` reads `r.svc.RuntimeState().DatabasePath`
     instead of `r.svc.Config().Database.Path`, and
     `r.svc.RuntimeState().ActiveLedger` instead of
     `r.svc.Config().ActiveLedger`.
5. **`internal/app/app_test.go`** — the poll-loop in
   `TestApp_WatchSwapsStoreOnExternalSwitch` switches from reading
   `a.cfg.*` to `a.RuntimeState()`. Tests that pre-set
   `cfg.ActiveLedger = "a"` (lines 56, 186) are deleted; tests
   asserting `a.cfg.ActiveLedger` / `a.cfg.Database.Path` (lines
   79-83, 107-111, 140-144, 224-236) move to `a.RuntimeState()`.
6. **`cmd/info_test.go`** — does not exist today. The new test below
   (`TestInfoRunner_UsesRuntimeStateNotConfig`) is the first test
   file for this command and introduces a fake `InfoProvider`.

### Out of scope

- The `~` expansion at `cmd/root.go:317-322` (touches Config at
  startup, not runtime — different lifecycle).
- Any change to `*ledger.Registry`'s public API.
- Any change to `/api/ledgers/*` handlers — they already read from
  the registry, not from cfg.

## Testing

### New tests in `internal/app/app_test.go`

- `TestApp_RuntimeStateInitialFromRegistry` — table-driven:
  construct `App` with a registry whose active entry is
  `{name: "alpha", path: "/tmp/a.db"}`; assert `app.RuntimeState()`
  returns the matching pair without any pre-set cfg fields.
- `TestApp_RuntimeStateAfterSwitchLedger` — call
  `SwitchLedger("b")`; assert `RuntimeState()` reflects b's name and
  path, and `cfg.Database.Path` is unchanged from its yaml-loaded
  value (proves the decoupling).
- `TestApp_RuntimeStateRace` — spawn a goroutine that calls
  `RuntimeState()` in a tight loop; trigger an external file change
  via the watcher; assert no race under `-race` and that the final
  state matches.

### Updated tests

- `TestApp_WatchSwapsStoreOnExternalSwitch` — the poll-loop reads
  `a.RuntimeState()` instead of `a.cfg.*`. Race-free by construction.
- The "no leak on Swap failure" assertions at lines 107-111, 140-144
  move to `a.RuntimeState()`. Invariant unchanged: a failed swap
  must not advance runtime state.

### New CLI test

`TestInfoRunner_UsesRuntimeStateNotConfig` — a fake `InfoProvider`
returns one path in `Config().Database.Path` and a different one in
`RuntimeState().DatabasePath`; assert the rendered output uses the
runtime one.

### Verification

Must pass before completion:

```bash
go test ./...
go test -race ./...
go build ./...
```

The canary is
`go test -race ./internal/app/ -run TestApp_RuntimeStateRace`.

## Risk / non-risk

- **Risk:** call-sites outside this file set that read
  `cfg.ActiveLedger` or `cfg.Database.Path` would break. The grep at
  brainstorm time showed only `cmd/info.go`, `cmd/root.go`,
  `internal/app/app.go`, and their tests touch these fields. No
  API handler reads them. No service-layer code reads them.
- **Non-risk:** `Database.Path` continues to exist on Config, so
  yaml unmarshalling and the startup `~`-expansion are unaffected.
  Users' `config.yaml` files are not touched.
