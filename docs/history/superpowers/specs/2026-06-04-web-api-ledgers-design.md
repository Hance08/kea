# Web API Ledgers Endpoints

**Date:** 2026-06-04
**Status:** Approved design — ready for implementation plan
**Scope:** REST endpoints for listing, creating, switching, and deleting ledgers. Reconciliation, the SPA, and ledger renaming are out of scope.

## Context

The foundation spec ([`2026-06-02-web-api-foundation-design.md`](2026-06-02-web-api-foundation-design.md)), read-endpoints spec ([`2026-06-03-web-api-read-endpoints-design.md`](2026-06-03-web-api-read-endpoints-design.md)), and write-endpoints spec ([`2026-06-03-web-api-write-endpoints-design.md`](2026-06-03-web-api-write-endpoints-design.md)) landed the foundation plus 19 endpoints on accounts, transactions, and reports.

This spec adds the ledger surface — five endpoints that let the SPA list, create, switch, and delete the named SQLite databases tracked in `ledgers.yaml`.

The pre-development review ([`docs/web-layer/2026-06-02-pre-development-review.md`](../../web-layer/2026-06-02-pre-development-review.md), section 5) sketched only `GET /api/ledgers` and `POST /api/ledgers/switch` and called the "CLI ledger switch invisible to running web server" issue (#119) a known limitation. That limitation is in fact already addressed in `internal/app/app.go`: `NewApp` registers an `OnSwitch` callback that calls `dbStore.Swap` and updates `cfg`, and `registry.Watch` fires the callback via fsnotify (with a 100ms debounce) when `ledgers.yaml` changes. This spec piggybacks on that wiring and adds a synchronous switch path for API-initiated changes that bypasses the debounce.

## Decisions

| Concern | Choice | Rationale |
|---------|--------|-----------|
| Endpoint scope | Full CLI parity — list, active, create, switch, delete | The SPA can manage ledger lifecycle end-to-end without dropping to the terminal. |
| Switch semantics | Synchronous in-process swap | Handler returns 200 only after `dbStore.Swap` completes, so the SPA's next request hits the new ledger. Avoids the ~100ms fsnotify debounce that watcher-only switches would expose. |
| Create body | `{"name": "..."}` only — always default location | DB file goes to `<appDir>/ledgers/<name>.db`. Keeps the HTTP surface from being able to write SQLite files to arbitrary paths on disk. |
| Delete behavior | Unregister only — DB file stays | Matches `kea ledger remove` without `--delete-file`. CLI keeps the destructive option. |
| Wiring approach | Extend `NewServer` signature (registry, migrations, appDir, switchFn) | One new method (~15 LOC) on `*app.App` for the synchronous switch. No new abstraction layer. |
| Path exposure | Include filesystem path in `LedgerInfo` | Parity with `kea ledger list`. Acceptable under the local single-user assumption. |
| New deps / sentinels / service methods | None | All four registry sentinels (`ErrLedgerNotFound`, `ErrLedgerExists`, `ErrRemoveActive`, `ErrNoActiveLedger`) already exist; this spec wires them through `mapError`. |

## Endpoint Catalog

| Method | Path | Body / Param | Returns |
|--------|------|--------------|---------|
| GET | `/api/ledgers` | — | `{"active": "name", "items": [LedgerInfo, ...]}` |
| GET | `/api/ledgers/active` | — | `LedgerInfo` (404 if none) |
| POST | `/api/ledgers` | `{"name": "..."}` | `201` `LedgerInfo` |
| POST | `/api/ledgers/switch` | `{"name": "..."}` | `200` `LedgerInfo` (newly active) |
| DELETE | `/api/ledgers/{name}` | — | `200 {"deleted": true, "name": N}` |

### Mechanical notes

- `GET /api/ledgers` returns `items` sorted by name (matches `registry.Names()`); `active` is the current active name, empty string when none.
- `POST /api/ledgers/switch` accepts the target in the body (symmetric with create) rather than as a path param, and explicitly does not return 202 — the response only resolves after the synchronous swap.
- Chi resolves the static `/ledgers/active` and `/ledgers/switch` routes before the parametric `/ledgers/{name}`. A ledger literally named `active` or `switch` would still be creatable but unreachable via the DELETE route. Acceptable edge case; not enforced via reserved-name validation.
- DELETE always unregisters only; the `.db` file remains on disk. CLI's `kea ledger remove --delete-file` retains the destructive option.
- All four ledger sentinels become exercisable via the API; tests pin each to the documented status code.

## Request and Response Shapes

### API-local DTOs — `internal/api/ledgers.go`

```go
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
```

`createLedgerRequest` and `switchLedgerRequest` are kept as distinct types so either endpoint can grow new fields without coupling.

`ledger.Entry` (the YAML struct) carries only the path; the API DTO adds `Name` and `Active` for SPA convenience. No JSON tags on `ledger.Entry` itself — the API layer owns the wire shape.

### Response examples

```json
GET /api/ledgers
{
  "active": "default",
  "items": [
    { "name": "default", "path": "/Users/.../kea/ledgers/default.db", "active": true },
    { "name": "personal", "path": "/Users/.../kea/ledgers/personal.db", "active": false }
  ]
}

GET /api/ledgers/active
{ "name": "default", "path": "/Users/.../kea/ledgers/default.db", "active": true }

POST /api/ledgers/switch  body: {"name": "personal"}
{ "name": "personal", "path": "/Users/.../kea/ledgers/personal.db", "active": true }

DELETE /api/ledgers/personal
{ "deleted": true, "name": "personal" }
```

### Error body (foundation, unchanged)

```json
{ "error": "cannot_remove_active", "message": "cannot remove active ledger — switch to another ledger first" }
```

## Error Mapping

Four branches added to `mapError` in `internal/api/errors.go`:

| Sentinel | Status | Error code |
|----------|--------|------------|
| `ledger.ErrLedgerNotFound` | 404 | `not_found` |
| `ledger.ErrLedgerExists` | 409 | `already_exists` |
| `ledger.ErrRemoveActive` | 409 | `cannot_remove_active` |
| `ledger.ErrNoActiveLedger` | 404 | `no_active_ledger` |

`not_found` and `already_exists` reuse the existing service-level codes — the semantics overlap, and reusing the code keeps the SPA's error-handling switch short. `cannot_remove_active` and `no_active_ledger` are new codes because the SPA may want to render targeted messages (e.g. "switch first" vs "create a ledger").

Each branch uses `errors.Is` so a sentinel wrapped via `fmt.Errorf("...: %w", ...)` still routes correctly. The four wrap-through cases are covered by `errors_test.go`.

### Validation

Mapped to `*service.ValidationError` → 400:

- Empty `name` on create or switch → `field: "name"`, message `"name is required"`.
- `name` containing `/`, `\`, `..`, or an ASCII control character → `field: "name"`, message `"name must not contain path separators"`. Defense-in-depth — the default-location-only design already prevents traversal, but explicit rejection beats relying on `filepath.Join` behavior.

DELETE's path param: chi will not match an empty segment, so no explicit empty-name check is required for that endpoint.

### Decode failures

Same machinery as existing write endpoints: `decodeJSON` with `DisallowUnknownFields()` returns `*service.ValidationError{Field: "body"}` → 400 for unknown fields, malformed JSON, or empty body.

## Handler Structure

### New method on `*app.App` — `internal/app/app.go`

```go
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

Requires three new fields on `App`: `store *store.Store`, `migrations fs.FS`, `cfg *config.Config`. All are already in scope in `NewApp`; the change is mechanical.

**Order rationale:** swap first, then YAML write. If `Swap` fails, no observable state has changed. If `Registry.Switch` fails after `Swap` succeeds, in-memory state is on the new ledger but disk says otherwise; the error is returned and surfaced. Restart converges. Acceptable for local single-user.

**Watcher interaction:** `Registry.Switch(name)` writes YAML, which triggers the fsnotify watcher's debounced reload. Inside `reload()`, `if fresh.ActiveLedger == prev` short-circuits before firing `OnSwitch` callbacks — and by the time fsnotify fires, `r.ActiveLedger` already equals `name` (set by `Switch`). The watcher path becomes a no-op, so no double-swap. CLI-driven switches still flow through `OnSwitch` normally; this is the path that handles `kea ledger switch` while the server is running.

### `internal/api/server.go` changes

```go
type Server struct {
    cfg        *config.Config
    svc        *service.Service
    registry   *ledger.Registry
    appDir     string
    migrations fs.FS
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
) *Server { ... }
```

`cmd/serve.go` wires:

```go
srv := api.NewServer(
    cfg, application.Service,
    application.Registry, migrations, appDir,
    application.SwitchLedger,
    logger,
)
```

`migrations` and `appDir` are already plumbed into `cmd/root.go` for `NewLedgerCmd`; `serve.go` picks them up the same way.

### Handlers — `internal/api/ledgers.go`

Each handler is a method on `*Server`, under ~20 lines, matching the existing convention.

```go
func (s *Server) handleListLedgers(w http.ResponseWriter, r *http.Request) error {
    active := s.registry.ActiveName()
    names := s.registry.Names()
    items := make([]ledgerInfo, 0, len(names))
    for _, n := range names {
        e, _ := s.registry.EntryFor(n)
        items = append(items, ledgerInfo{Name: n, Path: e.Path, Active: n == active})
    }
    return writeJSON(w, 200, ledgerListResponse{Active: active, Items: items})
}

func (s *Server) handleActiveLedger(w http.ResponseWriter, r *http.Request) error {
    name := s.registry.ActiveName()
    if name == "" {
        return ledger.ErrNoActiveLedger
    }
    e, ok := s.registry.EntryFor(name)
    if !ok {
        return fmt.Errorf("%w: %q", ledger.ErrLedgerNotFound, name)
    }
    return writeJSON(w, 200, ledgerInfo{Name: name, Path: e.Path, Active: true})
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
    return writeJSON(w, 201, ledgerInfo{Name: req.Name, Path: dbPath, Active: false})
}

func (s *Server) handleSwitchLedger(w http.ResponseWriter, r *http.Request) error {
    var req switchLedgerRequest
    if err := decodeJSON(r, &req); err != nil {
        return err
    }
    if err := validateLedgerName(req.Name); err != nil {
        return err
    }
    if err := s.switchFn(req.Name); err != nil {
        return err
    }
    e, _ := s.registry.EntryFor(req.Name)
    return writeJSON(w, 200, ledgerInfo{Name: req.Name, Path: e.Path, Active: true})
}

func (s *Server) handleDeleteLedger(w http.ResponseWriter, r *http.Request) error {
    name := chi.URLParam(r, "name")
    if err := s.registry.Remove(name, false); err != nil {
        return err
    }
    return writeJSON(w, 200, map[string]any{"deleted": true, "name": name})
}
```

`validateLedgerName` lives in the same file. Non-empty + rejects `/`, `\`, `..`, ASCII control chars. Returns `*service.ValidationError{Field: "name", ...}`.

### Concurrency / mid-flight requests

A switch invalidates any IDs the SPA holds from the previous ledger. The API does not detect or prevent this — `service` methods naturally return `ErrNotFound` or surface validation errors when an old ID is replayed against the new DB. The SPA refetches after switch.

`dbStore.Swap` is already mutex-guarded inside `store.Store`, so an in-flight request handler holding the old `*sql.DB` is sequenced safely against the swap.

### Router additions — `internal/api/router.go`

```go
r.Method("GET",    "/ledgers",         apiHandler(s.handleListLedgers))
r.Method("GET",    "/ledgers/active",  apiHandler(s.handleActiveLedger))
r.Method("POST",   "/ledgers",         apiHandler(s.handleCreateLedger))
r.Method("POST",   "/ledgers/switch",  apiHandler(s.handleSwitchLedger))
r.Method("DELETE", "/ledgers/{name}",  apiHandler(s.handleDeleteLedger))
```

## Testing

All tests in `internal/api/`, table-driven, `httptest` + stdlib only.

### Test substrate additions — `internal/api/testhelper_test.go`

The existing `newTestServer` was built for handlers that exercise the service layer over an in-memory SQLite. The ledger handlers don't touch the service layer — they touch the registry and a `switchFn`. One new helper:

```go
// newTestServerWithLedger wires a fresh Server with:
//   - a real *ledger.Registry rooted at t.TempDir()/ledgers.yaml
//   - migrations from the embedded FS
//   - appDir set to t.TempDir()
//   - a fake switchFn that records calls and updates registry.ActiveLedger
//     via registry.Switch(name), without invoking dbStore.Swap
func newTestServerWithLedger(t *testing.T) (
    ts *httptest.Server,
    reg *ledger.Registry,
    appDir string,
    switchCalls *[]string,
)
```

The API-side tests use the fake `switchFn` because the real store-swap behavior is locked in by an `internal/app/app_test.go` unit test below; duplicating it across both layers would not pin additional behavior.

### `internal/api/ledgers_test.go`

**`handleListLedgers`**
- Empty registry → 200, `{"active": "", "items": []}`.
- Two registered, one active → items sorted by name; exactly one has `"active": true`; `path` matches the seeded path.

**`handleActiveLedger`**
- Active set → 200 with `LedgerInfo`.
- No active → 404, `error: "no_active_ledger"`. **Exercises `ErrNoActiveLedger`.**

**`handleCreateLedger`**
- Valid name → 201, body matches `LedgerInfo{Name, Path, Active: false}`; DB file exists at `<appDir>/ledgers/<name>.db`; subsequent `GET /api/ledgers` includes it.
- Duplicate name → 409, `error: "already_exists"`. **Exercises `ErrLedgerExists`.**
- Empty name → 400, `field: "name"`.
- Name containing `/`, `\`, `..`, or `\x00` — one table row per character class → 400, `field: "name"`.
- Unknown JSON field → 400.

**`handleSwitchLedger`**
- Valid switch → 200, response reflects the now-active ledger; `GET /api/ledgers/active` confirms; `switchCalls` recorded the call exactly once.
- Switching to current active → 200; `switchFn` still invoked (semantics decision; idempotent on the fake), registry state unchanged.
- Unknown ledger → 404, `error: "not_found"`. **Exercises `ErrLedgerNotFound`** through the fake `switchFn`.
- Empty name → 400, `field: "name"`.

**`handleDeleteLedger`**
- Inactive ledger → 200 `{"deleted": true, "name": N}`; subsequent `GET /api/ledgers` omits it; the `.db` file still exists on disk (post-condition asserted).
- Active ledger → 409, `error: "cannot_remove_active"`. **Exercises `ErrRemoveActive`.**
- Unknown ledger → 404, `error: "not_found"`.

### `internal/api/errors_test.go` additions

One table row per new sentinel (`ErrLedgerNotFound`, `ErrLedgerExists`, `ErrRemoveActive`, `ErrNoActiveLedger`), plus one wrapped-with-`%w` case per sentinel to verify `errors.Is` propagation. Same shape as the existing rows.

### `internal/app/app_test.go` — new file

Unit test of `App.SwitchLedger` against a real `store.Store` over a `t.TempDir`-rooted registry. This is the only place the real swap is exercised; the API tests use a fake.

- Switch to a registered ledger → after the call, `store` reads from the new path (write a row to the new ledger, read it back), `cfg.ActiveLedger` reflects, `cfg.Database.Path` reflects, `registry.ActiveLedger` reflects, YAML on disk reflects.
- Switch to unknown name → returns `ErrLedgerNotFound`; cfg, store, registry, and YAML are all unchanged (snapshot before and after).
- Failed `Swap` (path with no migrations available, or invalid file) → returns the wrapped error; cfg, store, registry, and YAML unchanged. Pins the swap-first ordering.

### What earlier specs already cover (not re-tested)

- `apiHandler`, `writeJSON`, `decodeJSON`, `DisallowUnknownFields` — foundation tests.
- Sentinel-to-status mapping infrastructure — foundation `errors_test.go` (this spec adds rows, not new infrastructure).
- Middleware, request ID, access log, CORS, graceful shutdown — covered.

## Files Touched

**Added:**
- `internal/api/ledgers.go`
- `internal/api/ledgers_test.go`
- `internal/app/app_test.go`

**Modified:**
- `internal/app/app.go` — add `store`, `migrations`, `cfg` fields to `App`; add `SwitchLedger(name string) error` method.
- `internal/api/server.go` — extend `Server` struct and `NewServer` signature with `registry`, `migrations`, `appDir`, `switchFn`.
- `internal/api/router.go` — register five new routes.
- `internal/api/errors.go` — four `errors.Is` branches for the new sentinels; two new error codes (`cannot_remove_active`, `no_active_ledger`).
- `internal/api/errors_test.go` — table rows for the new sentinels.
- `internal/api/testhelper_test.go` — `newTestServerWithLedger` helper.
- `cmd/serve.go` — pass the new args into `NewServer`.

**Unchanged:**
- Service layer, repository, store, model, migrations, config struct.
- Existing API handler files (`accounts.go`, `transactions.go`, `reports.go`, `accounts_write.go`, `transactions_write.go`, `params.go`, `handler.go`, `middleware.go`, `health.go`) and their tests.
- CLI `cmd/ledger/*` files — they continue to operate on `*ledger.Registry` directly.

## Out of Scope

- Renaming a ledger — no `PATCH /api/ledgers/{name}`.
- Custom DB paths via the API (decided; default location only).
- `?delete_file=true` on DELETE (decided; unregister only).
- Optimistic concurrency / transition tokens on switch.
- Notifying connected SPA clients about CLI-driven switches (WebSocket / SSE). The SPA can poll `/api/ledgers/active` on focus or refetch after errors.
- The React SPA itself.
- Authentication, TLS, metrics, tracing, rate limiting — same as in earlier specs.
