# Web API Foundation

**Date:** 2026-06-02
**Status:** Approved design — ready for implementation plan
**Scope:** Backend API foundation only (no endpoint catalog, no frontend)

## Context

The Kea web layer is a React SPA + REST API that sits on the same `internal/service/` layer used by the existing CLI/TUI. Deployment is local-only, single-user — `kea serve` runs on `localhost` and shares the SQLite file with the CLI. The pre-development review ([`docs/web-layer/2026-06-02-pre-development-review.md`](../../web-layer/2026-06-02-pre-development-review.md)) confirmed the service layer is ready and listed open design questions.

This design covers the foundation layer only:

- `kea serve` cobra command
- Router and middleware chain
- Error mapping from service sentinels to HTTP responses
- JSON request/response helpers
- Two trivial endpoints (`/api/health`, `/api/version`) to exercise the stack
- Graceful shutdown wiring
- Test coverage for all of the above

The endpoint catalog (accounts, transactions, reports, reconcile) and the React SPA are each their own subsequent brainstorm → spec → plan cycle.

## Decisions

| Concern | Choice | Rationale |
|---------|--------|-----------|
| Router | `github.com/go-chi/chi/v5` | Idiomatic Go, std `net/http` handler signature, minimal surface, sub-router support. |
| Package layout | `internal/api/` (single package) | Matches existing `internal/service/`, `internal/store/` convention. Splits later if it grows. |
| Handler signature | `func(w, r) error` via adapter | Centralizes error-to-HTTP mapping. ~5 lines of adapter, every handler stays linear. |
| Logging | stdlib `log/slog`, JSON to stderr | No new dependency. Request-scoped logger via context. |
| CORS | Small custom middleware | Reads `cfg.Server.CORSOrigins`; no need for `rs/cors`. |
| Shutdown | Reuse `cmd/root.go`'s `signal.NotifyContext` | Already wired. `kea serve` blocks on `cmd.Context()`, then `srv.Shutdown(timeoutCtx)` with 5s grace. |
| Initial endpoints | `GET /api/health` and `GET /api/version` | Enough to verify the full stack end-to-end. |

## Package Layout

```
internal/api/
  server.go          // NewServer(cfg, svc, logger) → *Server; Run(ctx) error
  router.go          // routes() — chi router assembly, mounts /api subrouter
  handler.go         // apiHandler adapter type + writeJSON/decodeJSON helpers
  errors.go          // writeError(w, r, err) — sentinel-to-status mapping
  middleware.go      // requestID, accessLog, cors, logger
  health.go          // GET /api/health, GET /api/version handlers
  *_test.go          // table-driven tests using httptest

cmd/
  serve.go           // new cobra command: kea serve
```

Each file targets under ~150 lines. The package can split (`internal/api/middleware/`, `internal/api/handlers/`) when domain handlers land in later specs.

## Server Lifecycle

### `internal/api/server.go`

```go
type Server struct {
    cfg    *config.Config
    svc    *service.Service
    logger *slog.Logger
    http   *http.Server
}

func NewServer(cfg *config.Config, svc *service.Service, logger *slog.Logger) *Server {
    addr := net.JoinHostPort(cfg.Server.Host, strconv.Itoa(cfg.Server.Port))
    s := &Server{cfg: cfg, svc: svc, logger: logger}
    s.http = &http.Server{
        Addr:              addr,
        Handler:           s.routes(),
        ReadHeaderTimeout: 5 * time.Second,
    }
    return s
}

func (s *Server) Run(ctx context.Context) error {
    errCh := make(chan error, 1)
    go func() {
        s.logger.Info("server listening", "addr", s.http.Addr)
        if err := s.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
            errCh <- err
        }
        close(errCh)
    }()

    select {
    case err := <-errCh:
        return err
    case <-ctx.Done():
        shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        s.logger.Info("server shutting down")
        return s.http.Shutdown(shutdownCtx)
    }
}
```

`ListenAndServe` returns `http.ErrServerClosed` after `Shutdown`; that's not treated as an error.

### `cmd/serve.go`

```go
func NewServeCmd(svc *service.Service, cfg *config.Config) *cobra.Command {
    return &cobra.Command{
        Use:   "serve",
        Short: "Run the local web server",
        RunE: func(cmd *cobra.Command, args []string) error {
            logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
            srv := api.NewServer(cfg, svc, logger)
            return srv.Run(cmd.Context())
        },
    }
}
```

The constructor takes `*service.Service` and `*config.Config` the same way `NewAddCmd`, `NewReportCmd`, etc. do. `cmd/root.go` already builds the `*App` and registers each subcommand with `application.Service`; `serve` slots in alongside them:

```go
rootCmd.AddCommand(NewServeCmd(application.Service, cfg))
```

No new signal-handling code is needed — `cmd.Context()` is already cancelled on SIGINT/SIGTERM by `signal.NotifyContext` in `cmd/root.go`, and `rootCmd.ExecuteContext(ctx)` propagates that context to every subcommand.

### Backup and ledger-switch behavior

- `app.NewApp` runs the pre-startup backup. For `kea serve` this happens once at boot — the right behavior; server uptime won't trigger repeated backups.
- The known limitations from the pre-development review apply unchanged: CLI commands run during server uptime trigger their own backup paths (already WAL-aware after #80/#111), and CLI `ledger switch` is invisible to a running server (#119). Both are accepted as out of scope for local single-user use.

## Router and Middleware

### `internal/api/router.go`

```go
func (s *Server) routes() http.Handler {
    r := chi.NewRouter()

    r.Use(middleware.Recoverer)
    r.Use(requestIDMiddleware)
    r.Use(loggerMiddleware(s.logger))
    r.Use(accessLogMiddleware)
    r.Use(corsMiddleware(s.cfg.Server.CORSOrigins))

    r.Route("/api", func(r chi.Router) {
        r.Method("GET", "/health",  apiHandler(s.handleHealth))
        r.Method("GET", "/version", apiHandler(s.handleVersion))
    })

    return r
}
```

**Order rationale:** `Recoverer` outermost catches panics in everything below it. `requestID` and `logger` come next so all subsequent middleware and handlers see them. `accessLog` runs after them so its log line includes the request ID. `cors` is the last middleware before routes so `OPTIONS` preflight short-circuits without touching handlers.

### `internal/api/middleware.go`

Four middleware functions:

**`requestIDMiddleware`** — generates an 8-char hex ID (4 bytes from `crypto/rand`), sets the `X-Request-ID` response header, and stores the ID in the request context via a private key type. Exposes `requestIDFrom(ctx) string`.

**`loggerMiddleware(base *slog.Logger)`** — wraps the base logger with the `request_id` attribute and stores the wrapped logger in context. Exposes `loggerFrom(ctx) *slog.Logger`; returns a discard logger if none is set (so handlers never nil-deref).

**`accessLogMiddleware`** — wraps `http.ResponseWriter` in a small struct that captures the status code, runs the next handler, and emits one log line per request with `method`, `path`, `status`, `duration_ms`.

**`corsMiddleware(allowed []string)`** — reads the `Origin` header. If it's in `allowed`, sets `Access-Control-Allow-Origin: <origin>`, `Access-Control-Allow-Methods: GET, POST, PATCH, DELETE, OPTIONS`, `Access-Control-Allow-Headers: Content-Type`. For `OPTIONS` preflight, writes 204 and returns without calling `next`. If the origin is not allowed, no CORS headers are set and the request proceeds normally (so same-origin tools and `curl` are unaffected).

The default `cfg.Server.CORSOrigins` from the review work is `["http://localhost:5173"]` (Vite default).

## Handler Adapter and JSON Helpers

### `internal/api/handler.go`

```go
type apiHandler func(w http.ResponseWriter, r *http.Request) error

func (h apiHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    if err := h(w, r); err != nil {
        writeError(w, r, err)
    }
}

func writeJSON(w http.ResponseWriter, status int, v any) error {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    return json.NewEncoder(w).Encode(v)
}

func decodeJSON(r *http.Request, v any) error {
    dec := json.NewDecoder(r.Body)
    dec.DisallowUnknownFields()
    if err := dec.Decode(v); err != nil {
        return &service.ValidationError{Field: "body", Message: "invalid JSON: " + err.Error()}
    }
    return nil
}
```

`decodeJSON` wraps decode failures as `*service.ValidationError` so malformed request bodies route to 400 through the same machinery as service-layer validation errors. Foundation endpoints don't use `decodeJSON`, but the helper is in place for the next spec.

## Error Mapping

### `internal/api/errors.go`

```go
type errorBody struct {
    Error   string `json:"error"`
    Message string `json:"message"`
    Field   string `json:"field,omitempty"`
}

func writeError(w http.ResponseWriter, r *http.Request, err error) {
    status, body := mapError(err)
    loggerFrom(r.Context()).Error("request failed",
        "error_code", body.Error,
        "status", status,
        "err", err.Error(),
    )
    _ = writeJSON(w, status, body)
}

func mapError(err error) (int, errorBody) {
    var verr *service.ValidationError
    switch {
    case errors.As(err, &verr):
        return 400, errorBody{Error: "validation_failed", Message: verr.Message, Field: verr.Field}
    case errors.Is(err, service.ErrNotFound):
        return 404, errorBody{Error: "not_found",        Message: err.Error()}
    case errors.Is(err, service.ErrAlreadyExists):
        return 409, errorBody{Error: "already_exists",   Message: err.Error()}
    case errors.Is(err, service.ErrReconciled):
        return 409, errorBody{Error: "reconciled",       Message: err.Error()}
    case errors.Is(err, service.ErrCircularParent):
        return 409, errorBody{Error: "circular_parent",  Message: err.Error()}
    case errors.Is(err, service.ErrNotEditable):
        return 403, errorBody{Error: "not_editable",     Message: err.Error()}
    default:
        return 500, errorBody{Error: "internal", Message: "internal server error"}
    }
}
```

Adding a new sentinel later is a one-line change. The 500 branch deliberately hides the raw error text from the response body — it's logged in full — so internal failures don't leak implementation details across the API boundary.

## Health and Version Endpoints

### `internal/api/health.go`

```go
var Version = "dev" // override via -ldflags "-X github.com/hance08/kea/internal/api.Version=v0.1.0"

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) error {
    return writeJSON(w, 200, map[string]string{"status": "ok"})
}

func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) error {
    return writeJSON(w, 200, map[string]string{"version": Version})
}
```

Methods on `*Server` so future domain handlers can reach `s.svc`, `s.cfg`, `s.logger` the same way. Both return `error` to flow through `apiHandler`.

## Testing

All tests are table-driven and use stdlib `net/http/httptest`. No new test dependencies.

### `internal/api/router_test.go`

Boots a `Server` (passing a nil `*service.Service` — foundation endpoints don't touch it) and wraps `s.routes()` with `httptest.NewServer`. Cases:

- `GET /api/health` → 200, body `{"status":"ok"}`, `X-Request-ID` header non-empty.
- `GET /api/version` → 200, body `{"version":"dev"}`.
- `GET /api/unknown` → 404 (chi default).
- `OPTIONS /api/health` with `Origin: http://localhost:5173` → 204, `Access-Control-Allow-Origin: http://localhost:5173`.
- `OPTIONS /api/health` with `Origin: http://evil.example` → no `Access-Control-Allow-Origin` header.

### `internal/api/errors_test.go`

Pure unit test of `mapError`. Cases: each sentinel (`ErrNotFound`, `ErrAlreadyExists`, `ErrReconciled`, `ErrCircularParent`, `ErrNotEditable`), `*ValidationError`, an `ErrNotFound` wrapped with `fmt.Errorf("...: %w", ...)` (verifies `errors.Is` works through wrapping), and an unknown error → 500 with `Message: "internal server error"`.

### `internal/api/middleware_test.go`

- `requestIDMiddleware` attaches a non-empty ID retrievable via `requestIDFrom(ctx)` and sets `X-Request-ID` on the response.
- `accessLogMiddleware` captures the status code from a handler that writes 418, verified by parsing the JSON log line written to a `bytes.Buffer` handler.
- `loggerFrom` falls back to a non-nil discard logger when no middleware ran.

### `cmd/serve_test.go`

Minimal: build the command, run it with a `cmd.Context()` that's already cancelled, assert it returns `nil` (graceful shutdown path, no listening loop). Heavier end-to-end coverage is out of scope for the foundation spec.

## Out of Scope

These are explicitly deferred to subsequent specs:

- Domain endpoints (accounts, transactions, reports, reconcile).
- React SPA architecture and embedding strategy.
- Live reload between CLI and server when ledger switches (#119 known limitation).
- Authentication / multi-user (architectural assumption: local-only, single-user).
- TLS — `kea serve` is HTTP-only on `localhost`.
- Metrics, tracing, request rate limiting.

## Files Touched

**Added:**
- `cmd/serve.go`
- `internal/api/server.go`
- `internal/api/router.go`
- `internal/api/handler.go`
- `internal/api/errors.go`
- `internal/api/middleware.go`
- `internal/api/health.go`
- `internal/api/router_test.go`
- `internal/api/errors_test.go`
- `internal/api/middleware_test.go`
- `cmd/serve_test.go`

**Modified:**
- `cmd/root.go` — call `rootCmd.AddCommand(NewServeCmd(application.Service, cfg))` alongside the other `AddCommand` lines.
- `go.mod` / `go.sum` — add `github.com/go-chi/chi/v5`.
