# Web API Read Endpoints — Accounts, Transactions, Reports

**Date:** 2026-06-03
**Status:** Approved design — ready for implementation plan
**Scope:** Read-only domain endpoints. Writes, reconcile, and the SPA are out of scope.

## Context

The foundation spec ([`2026-06-02-web-api-foundation-design.md`](2026-06-02-web-api-foundation-design.md)) landed the router, middleware chain, `apiHandler` adapter, error-to-status mapping, and JSON helpers, plus two trivial endpoints (`/api/health`, `/api/version`) to exercise the full stack. The service layer was readied for this work during the pre-development review ([`docs/web-layer/2026-06-02-pre-development-review.md`](../../web-layer/2026-06-02-pre-development-review.md)).

This spec adds the read-only domain surface — twelve endpoints across accounts, transactions, and reports. Every endpoint maps onto an existing method on `svc.Account()` or `svc.Transaction()`. No new service methods, no new repo methods, no new error sentinels, no new dependencies.

## Decisions

| Concern | Choice | Rationale |
|---------|--------|-----------|
| Account lookup style | Dual — `/accounts/{id}` and `/accounts/by-name?name=...` | Account names contain `:` and may grow more reserved characters; query-string lookup keeps routing trivial. ID stays canonical. |
| Response envelope | Bare resources; `ListResult` for collections; bare arrays for the tree | Matches `internal/model/` 1:1, zero translation layer. |
| Query parameter style | `snake_case`, matching JSON tags | One consistent vocabulary across body and query. |
| Money serialization | `int64` cents, no formatting | The SPA formats. Same as the model layer. |
| Date params (reports) | `?month=YYYY-MM` or `?from=YYYY-MM-DD&to=YYYY-MM-DD`; snapshots take `?as_of=` / `?at=` Unix seconds | Reuses `DateRangeParams` directly. |
| Tx list payload | `TransactionDetail` (with splits) via bulk fetch | One extra JOIN per page; SPA renders amount/account without a second round-trip. |
| Unknown query params | Silently ignored | Lets the SPA append tracking params without breakage. |
| Filter routing | Single endpoint per resource | `/api/accounts` covers list + search + by-type. `/api/transactions` covers list + filter. |
| New deps | None | Stdlib + the foundation's chi router only. |

## Endpoint Catalog

### Accounts

| Method | Path | Query | Returns |
|--------|------|-------|---------|
| GET | `/api/accounts` | `type`, `q`, `currency`, `include_hidden`, `limit`, `offset`, `include_count` | `ListResult[*Account]` |
| GET | `/api/accounts/tree` | `include_hidden` | `[]*AccountNode` |
| GET | `/api/accounts/by-name` | `name` (required) | `*Account` |
| GET | `/api/accounts/{id}` | — | `*Account` |
| GET | `/api/accounts/{id}/balance` | — | `{account_id, amount, currency}` |

### Transactions

| Method | Path | Query | Returns |
|--------|------|-------|---------|
| GET | `/api/transactions` | `account_id`, `type`, `status`, `start_time`, `end_time`, `description`, `limit`, `offset`, `include_count` | `ListResult[*TransactionDetail]` |
| GET | `/api/transactions/{id}` | — | `*TransactionDetail` |

### Reports

| Method | Path | Query | Returns |
|--------|------|-------|---------|
| GET | `/api/reports/income-statement` | `month` or `from`+`to` | `*ReportResult` |
| GET | `/api/reports/income-breakdown` | `month` or `from`+`to` | `*ReportResult` |
| GET | `/api/reports/expense-breakdown` | `month` or `from`+`to` | `*ReportResult` |
| GET | `/api/reports/balance-sheet` | `as_of` (Unix seconds; default = now) | `*BalanceSheetResult` |
| GET | `/api/reports/net-worth` | `at` (Unix seconds; default = now) | `{at, net_worth}` |

### Mechanical notes

- Chi's static `/accounts/tree` and `/accounts/by-name` routes take precedence over `/accounts/{id}` because chi matches static segments first — no manual ordering required.
- `/api/transactions` filters by `account_id` only (not `account_name`). The SPA can resolve a name via `/api/accounts/by-name` first; this mirrors how `FilterTransactionsByAccountName` works in the service.
- When `?as_of=` or `?at=` is absent on snapshot reports, the handler defaults to `time.Now().Unix()`.

## Response Shapes

Bare resources, `ListResult` envelope for collections, no `{data: ...}` wrapper.

### `Account` (existing `model.Account`)
```json
{
  "id": 12,
  "name": "Assets:Bank:Checking",
  "type": "A",
  "parent_id": 5,
  "currency": "USD",
  "description": "",
  "is_hidden": false
}
```

### Account list — `ListResult[*Account]`
```json
{
  "items": [ /* Account */ ],
  "total_count": 42,
  "limit": 20,
  "offset": 0
}
```

When `GET /api/accounts` routes to `ListAccounts` (no filter / pagination intent), the handler still wraps the slice in `ListResult` with `total_count = len(items)`, `limit = 0`, `offset = 0`. The SPA sees one consistent shape.

### Account tree — `[]*AccountNode`
Bare array; hierarchy isn't paginated.
```json
[
  { "account": { /* Account */ },
    "children": [ { "account": { /* Account */ }, "children": [] } ] }
]
```

### Balance
```json
{ "account_id": 12, "amount": 125000, "currency": "USD" }
```
`amount` is int64 cents. `account_id` is included so the SPA can correlate when several balances are fetched in parallel.

### `TransactionDetail` (existing `model.TransactionDetail`)
```json
{
  "id": 88,
  "timestamp": 1733184000,
  "description": "Coffee",
  "status": "Cleared",
  "type": "Expense",
  "splits": [
    { "id": 201, "account_id": 12, "account_name": "Assets:Bank:Checking",
      "account_type": "A", "amount": -500, "currency": "USD", "memo": "" },
    { "id": 202, "account_id": 34, "account_name": "Expenses:Coffee",
      "account_type": "E", "amount": 500, "currency": "USD", "memo": "" }
  ]
}
```

### Transaction list — `ListResult[*TransactionDetail]`
The handler calls `FilterTransactions` (or `ListRecentTransactions`) for `[]*Transaction`, then `GetTransactionDetailsByIDs` for splits in one round trip, then assembles `[]*TransactionDetail` preserving the order returned by the service.

### Reports
Bare `model.ReportResult` and `model.BalanceSheetResult` — they already have JSON tags. Net-worth wraps the bare `map[string]int64` from `GetNetWorthAt`:
```json
{ "at": 1733184000, "net_worth": { "USD": 125000, "EUR": 30000 } }
```

### Error body (already in foundation)
```json
{ "error": "validation_failed", "message": "limit must be >= 0", "field": "limit" }
```

## Query Parameter Parsing & Validation

A small parsing layer turns query strings into the service-layer structs. Each parser returns either a value or a `*service.ValidationError`, which the existing `mapError` already routes to 400.

### `internal/api/params.go` — core helpers

```go
// Returns (nil, nil) when ?key is absent; *ValidationError on bad input.
parseInt64Query(r *http.Request, key string) (*int64, error)
parseIntQuery(r *http.Request, key string) (*int, error)

// Accepts "true"/"false"/"1"/"0"; returns (false, nil) if absent.
parseBoolQuery(r *http.Request, key string) (bool, error)

// Returns nil when absent or empty.
parseStringQuery(r *http.Request, key string) *string

// Path int64 — uses chi.URLParam, returns *ValidationError on parse failure.
parseInt64Path(r *http.Request, key string) (int64, error)
```

### `internal/api/params.go` — composite parsers

```go
// ?limit=&offset=&include_count=
//   limit  >= 0 (0 means "use service default")
//   offset >= 0
parseListOptions(r *http.Request) (model.ListOptions, error)

// ?type=&q=&currency=
//   type must be one of A/L/C/R/E if present
parseAccountFilter(r *http.Request) (model.AccountFilter, error)

// ?account_id=&type=&status=&start_time=&end_time=&description=
//   type   validated via TransactionType.IsValid()
//   status validated via ParseTransactionStatus (case-insensitive "Pending"/"Cleared"/"Reconciled")
//   if both start_time and end_time set, start <= end
parseTransactionFilter(r *http.Request) (model.TransactionFilter, error)

// ?month=&from=&to= — packed directly into DateRangeParams.
// Format/cross-field validation is deferred to ResolveDateRange,
// which already returns *ValidationError on bad input.
parseDateRangeParams(r *http.Request) service.DateRangeParams
```

### Conventions

- Absent params parse to `nil` (or zero), never default values at the API edge. Service-layer defaults stay authoritative.
- Empty strings (`?type=`) are treated as absent.
- Unknown query params are silently ignored.
- Every `ValidationError.Field` matches the query key the user typed (`"limit"`, `"start_time"`, `"type"`), so the error response maps directly back to the input.

## Handler Structure

Every handler is a method on `*Server` and stays under ~20 lines. Pattern:

```go
func (s *Server) handleListAccounts(w http.ResponseWriter, r *http.Request) error {
    ctx := r.Context()
    opts, err := parseListOptions(r)
    if err != nil { return err }
    filter, err := parseAccountFilter(r)
    if err != nil { return err }
    includeHidden, err := parseBoolQuery(r, "include_hidden")
    if err != nil { return err }

    if filter.Query != nil || filter.Type != nil || filter.Currency != nil ||
        opts.Limit > 0 || opts.Offset > 0 || opts.IncludeCount {
        res, err := s.svc.Account().SearchAccounts(ctx, filter, opts)
        if err != nil { return err }
        return writeJSON(w, 200, applyHiddenFilter(res, includeHidden))
    }

    accounts, err := s.svc.Account().ListAccounts(ctx,
        service.ListAccountsOptions{Type: filter.Type, ShowHidden: includeHidden})
    if err != nil { return err }
    return writeJSON(w, 200, wrapAsListResult(accounts))
}
```

Two file-local helpers:
- `wrapAsListResult(accounts []*model.Account) *model.ListResult[*model.Account]` — `{items, total_count: len(items), limit: 0, offset: 0}`.
- `applyHiddenFilter(res *ListResult, includeHidden bool)` — strips hidden items from `SearchAccounts` output (which doesn't filter hidden itself). Recomputes the result's length-based total when items were dropped.

### Handler list

**accounts.go**
- `handleListAccounts` — routes to `SearchAccounts` (when any filter / pagination param is present) or `ListAccounts`.
- `handleAccountTree` — parses `include_hidden`; calls `GetAccountTree`.
- `handleAccountByName` — reads `?name=`; 400 if empty; calls `GetAccountByName`.
- `handleAccountByID` — `parseInt64Path("id")`; calls `GetAccountByID`.
- `handleAccountBalance` — `parseInt64Path("id")`; calls `GetAccountBalance` plus `GetAccountByID` for currency; returns `{account_id, amount, currency}`.

**transactions.go**
- `handleListTransactions` — parses opts + filter; calls `FilterTransactions`, then `GetTransactionDetailsByIDs`; assembles `ListResult[*TransactionDetail]` preserving service-order. A zero-value filter routes through the same call — no branch needed.
- `handleTransactionByID` — `parseInt64Path("id")`; calls `GetTransactionByID`.

**reports.go**
- `handleIncomeStatement` — `parseDateRangeParams`; calls `GenerateFullIncomeStatement`.
- `handleIncomeBreakdown` / `handleExpenseBreakdown` — same shape, call their `Full*` variants.
- `handleBalanceSheet` — `parseInt64Query("as_of")`; defaults to `time.Now().Unix()` when absent; calls `GenerateBalanceSheet`.
- `handleNetWorth` — `parseInt64Query("at")`; defaults to now; calls `GetNetWorthAt`; wraps in `{at, net_worth}`.

## Router Additions

```go
r.Route("/api", func(r chi.Router) {
    r.Method("GET", "/health",  apiHandler(s.handleHealth))
    r.Method("GET", "/version", apiHandler(s.handleVersion))

    r.Method("GET", "/accounts",                apiHandler(s.handleListAccounts))
    r.Method("GET", "/accounts/tree",           apiHandler(s.handleAccountTree))
    r.Method("GET", "/accounts/by-name",        apiHandler(s.handleAccountByName))
    r.Method("GET", "/accounts/{id}",           apiHandler(s.handleAccountByID))
    r.Method("GET", "/accounts/{id}/balance",   apiHandler(s.handleAccountBalance))

    r.Method("GET", "/transactions",            apiHandler(s.handleListTransactions))
    r.Method("GET", "/transactions/{id}",       apiHandler(s.handleTransactionByID))

    r.Method("GET", "/reports/income-statement",  apiHandler(s.handleIncomeStatement))
    r.Method("GET", "/reports/income-breakdown",  apiHandler(s.handleIncomeBreakdown))
    r.Method("GET", "/reports/expense-breakdown", apiHandler(s.handleExpenseBreakdown))
    r.Method("GET", "/reports/balance-sheet",     apiHandler(s.handleBalanceSheet))
    r.Method("GET", "/reports/net-worth",         apiHandler(s.handleNetWorth))
})
```

## Error Mapping

No additions to `mapError`. The foundation already covers the read surface:

- `*ValidationError` → 400 — query parse failures, bad enum, bad path int, missing `?name=`.
- `ErrNotFound` → 404 — account/transaction lookups.
- The write-only sentinels (`ErrReconciled`, `ErrAlreadyExists`, `ErrCircularParent`, `ErrNotEditable`) aren't reachable from the read endpoints.

## Testing

All tests stay in `internal/api/`, table-driven, using `httptest` and stdlib only.

### Test substrate — `internal/api/testhelper_test.go`

Sets up a real `*service.Service` over an in-memory SQLite DB so handlers exercise the full stack end-to-end (handler → service → store). Foundation tests passed a nil `*service.Service` because health/version don't touch it; from here we need real data.

```go
func newTestServer(t *testing.T) (*httptest.Server, *service.Service) {
    t.Helper()
    cfg := config.Default()
    store, cleanup := newTestStore(t)  // file::memory:?cache=shared, migrations applied
    t.Cleanup(cleanup)
    svc := service.NewService(store, store, cfg)
    logger := slog.New(slog.NewTextHandler(io.Discard, nil))
    s := api.NewServer(cfg, svc, logger)
    ts := httptest.NewServer(s.Routes())
    t.Cleanup(ts.Close)
    return ts, svc
}

func seedAccounts(t *testing.T, svc *service.Service, ...) []*model.Account
func seedTransaction(t *testing.T, svc *service.Service, ...) *model.TransactionDetail
```

`Routes()` is a one-line exported method (`func (s *Server) Routes() http.Handler { return s.routes() }`) added so tests can wire a fresh handler. Production code still uses the internal `s.routes()` via `NewServer`.

If wiring a real store proves heavier than expected during implementation (test-isolation collisions on shared in-memory DBs, migrations cost), the plan step can fall back to lightweight per-test mocks of `service.AccountService` / `service.TransactionService`. The handler shape doesn't change either way.

### Per-handler tests

**`accounts_test.go`**
- `handleListAccounts`:
  - empty store → 200, `items: []`, `total_count: 0`.
  - mixed types + hidden accounts → default call returns visible only; `?include_hidden=true` returns all; `?type=A` filters by type; `?q=Bank&include_count=true` exercises the `SearchAccounts` path with pagination metadata.
  - `?limit=abc` → 400, `field: "limit"`.
  - `?type=Z` → 400, `field: "type"`.
- `handleAccountTree` — parent + child + hidden child; asserts hierarchy and that the default prunes hidden.
- `handleAccountByID` — existing → 200; missing → 404; non-numeric path → 400.
- `handleAccountByName` — existing → 200; missing `?name=` → 400, `field: "name"`; URL-encoded `Assets:Bank:Checking` round-trips.
- `handleAccountBalance` — seeds an account with two posted splits, asserts `{account_id, amount, currency}` with expected int64 sum. Missing ID → 404.

**`transactions_test.go`**
- `handleListTransactions`:
  - empty store → 200, `items: []`.
  - three seeded transactions → unfiltered call returns all in `timestamp DESC, id DESC` order; each item is a `TransactionDetail` with splits populated.
  - `?account_id=N` returns only transactions touching that account.
  - `?type=Expense&status=Cleared` combined filter.
  - `?start_time=X&end_time=Y` date range.
  - `?limit=2&offset=1&include_count=true` pagination meta.
  - `?status=Wat` → 400, `field: "status"`.
  - `?start_time=200&end_time=100` → 400, cross-field validation.
- `handleTransactionByID` — existing → 200 with splits; missing → 404; non-numeric → 400.

**`reports_test.go`**
- For each of `income-statement`, `income-breakdown`, `expense-breakdown`:
  - seed one income tx and one expense tx in a known month → 200 with non-empty rows; verify `period` label is set; verify totals.
  - `?month=2026-13` → 400, `field: "month"`.
  - `?from=2026-02-01&to=2026-01-01` → 400, `field: "to"`.
- `handleBalanceSheet` — seed an asset and a liability → 200; assert `as_of` defaults to ~now (within a tolerance window).
- `handleNetWorth` — seed two assets + one liability → 200 with `{at, net_worth: {USD: ...}}`; absent `?at` defaults to now.

**`params_test.go`** — pure unit tests of every parser. Each composite parser gets cases for: all absent, each field set individually, each invalid form (bad int, bad bool, bad enum), and cross-field validation.

### What the foundation tests already cover (not re-tested)

- `apiHandler` adapter, `writeJSON`, error-to-status mapping per sentinel — covered by `errors_test.go` and `router_test.go`.
- Middleware (request ID, access log, CORS, logger context) — covered by `middleware_test.go`.
- Graceful shutdown — covered by `server_test.go`.

## Files Touched

**Added:**
- `internal/api/params.go`
- `internal/api/accounts.go`
- `internal/api/transactions.go`
- `internal/api/reports.go`
- `internal/api/params_test.go`
- `internal/api/accounts_test.go`
- `internal/api/transactions_test.go`
- `internal/api/reports_test.go`
- `internal/api/testhelper_test.go`

**Modified:**
- `internal/api/router.go` — register the new handlers.
- `internal/api/server.go` — add the one-line `Routes()` exported wrapper around `routes()` for tests.

**Unchanged:**
- `internal/api/handler.go`, `errors.go`, `middleware.go`, `health.go`, `server_test.go`, `errors_test.go`, `middleware_test.go`, `router_test.go`.
- Every package outside `internal/api/`. No service, repo, model, or config changes.

## Out of Scope

These are explicitly deferred:

- Writes — `POST` / `PATCH` / `DELETE` on accounts and transactions.
- Reconcile endpoints (unreconciled list, preview, commit) — their own spec alongside the write spec.
- Ledger switch endpoint and the split-brain limitation (#119).
- Bulk balances endpoint (`GET /api/balances?as_of=`) — can be added when a SPA dashboard needs it.
- The React SPA itself.
- Authentication, TLS, metrics, tracing, rate limiting — same as in the foundation spec.
