# `GET /api/balances` — Bulk Account-Balance Snapshot

**Status:** Design approved 2026-06-05.
**Predecessor:** [2026-06-03-web-api-read-endpoints-design.md](2026-06-03-web-api-read-endpoints-design.md) deferred this endpoint pending SPA-side requirements.
**Use case:** SPA dashboard primary fetch — one round-trip returns every account with its current (or historical) balance, ready for the SPA to render however it wants.

## Problem

The current read API can produce a balance for a single account at a time (`GET /api/accounts/{id}/balance`). A dashboard rendering N accounts requires N round-trips, which is slow and wasteful when the data is already available in one repository call (`AccountRepository.GetAllAccountBalances(ctx, asOf int64) (map[int64]int64, error)`, defined at [internal/repository/interfaces.go:20](../../internal/repository/interfaces.go)).

`AccountService` does not currently expose `GetAllAccountBalances` — only the report layer's `GenerateBalanceSheet` uses it internally ([internal/service/report_service.go:293](../../internal/service/report_service.go)). The endpoint and a thin service wrapper are the deliverables.

## Decision

Ship a single read endpoint that returns a flat list of `(account_id, name, type, parent_id, currency, amount, is_hidden)` rows, wrapped in the existing `ListResult` shape. Defaults match the rest of the read API: hidden accounts excluded, `as_of` defaults to now, no pagination.

### Why flat list (vs grouped-by-type vs tree)

- Matches [`GET /api/accounts`](../../internal/api/accounts.go) — same `ListResult` envelope, same `wrapAsListResult` idiom.
- Maximally flexible for the SPA: it can group by type, build a tree by joining `parent_id`, sort by amount, or filter however the dashboard view demands. Server-side grouping would lock the SPA into one layout.
- A tree-shaped variant would duplicate `/api/accounts/tree`, which the SPA can already call and join with balance rows by `account_id`. Avoids redundant endpoints.

### Why no per-currency totals embedded

- The SPA has every row and can compute per-currency sums trivially.
- Different dashboard views (Assets-only, by-type, by-parent) want different aggregations; baking one into the response locks in a layout.
- `GenerateBalanceSheet` already computes per-currency totals when the use case is reporting; for raw balance access we keep the contract narrow.

### Why no pagination

- Personal-accounting account counts are tens to hundreds. Even a power user with 500 accounts is well within a single response.
- The `ListResult` envelope has `limit`/`offset` fields, but `wrapAsListResult` already sets them to `0` to signal "no pagination intent" — the same convention this endpoint adopts.
- If a future use case ever needs pagination, the envelope already supports it; we just don't wire query params now.

## Design

### Model

New struct in [internal/model/account.go](../../internal/model/account.go), placed next to `Account` and `AccountNode`:

```go
// AccountBalance is one row in a bulk-balance snapshot response.
// Joins per-account metadata (from Account) with a point-in-time balance.
type AccountBalance struct {
    AccountID int64       `json:"account_id"`
    Name      string      `json:"name"`
    Type      AccountType `json:"type"`
    ParentID  *int64      `json:"parent_id,omitempty"`
    Currency  string      `json:"currency"`
    Amount    int64       `json:"amount"`
    IsHidden  bool        `json:"is_hidden"`
}
```

`Currency` is the resolved currency: each account's own currency if set, else the config default (mirroring `GenerateBalanceSheet`'s normalization at [report_service.go:309](../../internal/service/report_service.go)). `Amount` is signed int64 cents — same convention as everywhere else in the codebase.

### Service method

New public method on `AccountService` (in [internal/service/account_service.go](../../internal/service/account_service.go), next to `GetAccountBalance`):

```go
// GetAccountBalancesBulk returns one AccountBalance row per account known to
// the registry, with the balance computed as of asOf (Unix seconds). When
// includeHidden is false, accounts with IsHidden=true are omitted. Rows are
// sorted by Account.ID for deterministic output.
func (as *AccountService) GetAccountBalancesBulk(
    ctx context.Context, asOf int64, includeHidden bool,
) ([]model.AccountBalance, error)
```

Implementation:

1. `accounts, err := as.repo.GetAllAccounts(ctx)` — full metadata. On error: wrap with `fmt.Errorf("load accounts: %w", err)`.
2. `balances, err := as.repo.GetAllAccountBalances(ctx, asOf)` — id → cents. On error: wrap with `fmt.Errorf("load balances: %w", err)`.
3. Allocate `rows := make([]model.AccountBalance, 0, len(accounts))`.
4. For each `acc`:
   - If `!includeHidden && acc.IsHidden`: continue.
   - `currency := acc.Currency; if currency == "" { currency = as.config.Defaults.Currency }`.
   - Append `AccountBalance{AccountID: acc.ID, Name: acc.Name, Type: acc.Type, ParentID: acc.ParentID, Currency: currency, Amount: balances[acc.ID], IsHidden: acc.IsHidden}`.
5. `sort.Slice(rows, func(i, j int) bool { return rows[i].AccountID < rows[j].AccountID })`.
6. Return `rows, nil`. Empty registry returns `[]model.AccountBalance{}` (not nil) so JSON marshaling gives `"items": []`.

Account-not-in-balances-map → zero balance (Go map default). This is the right behavior: a newly-created account with no transactions has zero balance, and the repo legitimately omits it from the map.

### Handler

New handler in [internal/api/accounts.go](../../internal/api/accounts.go), next to `handleAccountBalance`:

```go
func (s *Server) handleListBalances(w http.ResponseWriter, r *http.Request) error {
    asOfPtr, err := parseInt64Query(r, "as_of")
    if err != nil {
        return err
    }
    asOf := time.Now().Unix()
    if asOfPtr != nil {
        asOf = *asOfPtr
    }
    includeHidden, err := parseBoolQuery(r, "include_hidden")
    if err != nil {
        return err
    }
    rows, err := s.svc.Account().GetAccountBalancesBulk(r.Context(), asOf, includeHidden)
    if err != nil {
        return err
    }
    return writeJSON(w, http.StatusOK, &model.ListResult[model.AccountBalance]{
        Items:      rows,
        TotalCount: len(rows),
        Limit:      0,
        Offset:     0,
    })
}
```

No new parse helper needed. `parseInt64Query` ([internal/api/params.go:16](../../internal/api/params.go)) already returns `*int64` (nil for missing, `*ValidationError{Field: "as_of"}` for malformed). `parseBoolQuery` ([internal/api/params.go:40](../../internal/api/params.go)) handles `include_hidden`.

The `time.Now().Unix()` call happens in the handler, not the service — keeps the service deterministic for testing.

### Route registration

One line in [internal/api/router.go](../../internal/api/router.go) (after the existing `/accounts/{id}/balance` route at line 32):

```go
r.Method(http.MethodGet, "/balances", apiHandler(s.handleListBalances))
```

Placement matches the read-endpoints PR's convention (read endpoints grouped, write endpoints follow).

### Endpoint contract

| Aspect | Value |
|---|---|
| Method | `GET` |
| Path | `/api/balances` |
| Query: `as_of` | Optional. Unix seconds (`int64`). Omitting or empty resolves to server's `time.Now().Unix()` at request time. Non-integer → 400, `field: "as_of"`. |
| Query: `include_hidden` | Optional. `true`/`false`/`1`/`0`/empty. Default `false`. Other values → 400, `field: "include_hidden"`. |
| Success | `200 OK` + JSON body matching `ListResult[AccountBalance]`. |
| Empty registry | `200 OK` + `{"items": [], "total_count": 0, "limit": 0, "offset": 0}`. |

Sample response:

```json
{
  "items": [
    {"account_id": 1, "name": "Assets:Bank",     "type": "A", "currency": "USD", "amount": 125000, "is_hidden": false},
    {"account_id": 2, "name": "Assets:Cash",     "type": "A", "currency": "USD", "amount":   3500, "is_hidden": false},
    {"account_id": 5, "name": "Expenses:Coffee", "type": "E", "parent_id": 4, "currency": "USD", "amount":  -1850, "is_hidden": false}
  ],
  "total_count": 3,
  "limit": 0,
  "offset": 0
}
```

### Error handling

| Cause | Status | Body |
|---|---|---|
| `as_of` not an integer | 400 | `{error: "validation_failed", field: "as_of", message: "..."}` |
| `include_hidden` not a bool | 400 | `{error: "validation_failed", field: "include_hidden", message: "..."}` |
| `GetAllAccounts` / `GetAllAccountBalances` returns error | 500 | default `mapError` branch — internal-error envelope |

No 404 case — the endpoint is path-independent. An empty registry is a successful empty list, not a 404.

## Tests

### Service layer — `internal/service/account_service_test.go`

Add `TestGetAccountBalancesBulk` with table-driven subtests:

1. **Happy path, hidden excluded by default**: 4 accounts (1 hidden), balances map covers 3. Assert 3 rows (hidden omitted), sorted by ID, amounts match the map.
2. **Hidden included when toggled**: same input, `includeHidden=true`. Assert all 4 rows.
3. **Empty currency normalized to config default**: account with `Currency=""`, config default `"USD"`. Assert row's `Currency == "USD"`.
4. **Account not in balances map → zero**: 2 accounts, balances map has only 1. Assert the missing one returns row with `Amount=0`.
5. **Empty registry**: no accounts. Assert `[]AccountBalance{}` (non-nil, zero length).
6. **`GetAllAccounts` error propagates**: assert wrapped error.
7. **`GetAllAccountBalances` error propagates**: assert wrapped error.

Uses existing `mockAccountRepo` plus the existing `as_of`-aware `GetAllAccountBalances` mock (already at [testhelper_test.go:150](../../internal/service/testhelper_test.go)). The mock takes and ignores `asOf`; that's fine — `asOf` is forwarded to the repo, not used by the service. The test asserts forwarding via the existing recorder pattern if the mock supports it; otherwise the test just verifies the row shape.

### API layer — `internal/api/accounts_test.go`

Add the following functions:

- **`TestHandleListBalances_OK`** — seed 2 accounts (different types, different currencies), assert 200 + full envelope shape + `total_count=2` + rows sorted by ID.
- **`TestHandleListBalances_AsOfDefault`** — omit `as_of`, assert 200. Do *not* assert the exact timestamp — the service doesn't expose what it was called with, and we accept this gap deliberately (no `nowFunc` test seam).
- **`TestHandleListBalances_InvalidAsOf`** — `?as_of=notanumber` → 400, `error="validation_failed"`, `field="as_of"`.
- **`TestHandleListBalances_InvalidIncludeHidden`** — `?include_hidden=maybe` → 400, `error="validation_failed"`, `field="include_hidden"`.
- **`TestHandleListBalances_IncludeHidden`** — seed one hidden account; default excludes it, `?include_hidden=true` includes it.
- **`TestHandleListBalances_HistoricalAsOf`** — `?as_of=1700000000` returns 200; with mock service or service+store path, verify the request reaches the service with that value (use whatever test seam the read-endpoints PR already provides — typically `newServerWithStore` plus seeded data).
- **`TestHandleListBalances_Empty`** — fresh store with zero non-system accounts, assert 200 + `items: []` + `total_count: 0`. (System accounts like `Equity:OpeningBalances_USD` count as accounts; if the seed always creates them, adjust the assertion to "only the system account is present" or seed-and-clear as needed.)

The existing test harness (`newServerWithStore`, `seedAccount`) is the same one used by the write-endpoints tests; reuse it.

## Out of scope

- **Per-currency totals in the response.** SPA computes from rows. Deferred indefinitely — would need a separate use case to justify.
- **Pagination query params.** Account counts are bounded; YAGNI. The `limit`/`offset` envelope fields stay at `0` to signal "no pagination intent" (matches `wrapAsListResult`).
- **Tree-shaped balances.** SPA joins `/api/accounts/tree` with `/api/balances` rows by `account_id` if it needs hierarchy.
- **`POST /api/balances:batch` for arbitrary subsets of account IDs.** Different use case; no SPA requirement yet.
- **Repo-layer changes.** `GetAllAccountBalances` exists with the right signature; no new repo method.
- **`nonzero_only` filter.** Default is "include all"; SPA can hide zero rows client-side. Add later if dashboard polish requires it.

## Verification

- `go test ./internal/service/... -run TestGetAccountBalancesBulk` — green.
- `go test ./internal/api/... -run TestHandleListBalances` — green.
- `go test ./...` and `go build ./...` — green.
- Manual smoke (optional):
  - `curl localhost:<port>/api/balances` — default response.
  - `curl 'localhost:<port>/api/balances?include_hidden=true'` — hidden visible.
  - `curl 'localhost:<port>/api/balances?as_of=1700000000'` — historical snapshot.
  - `curl 'localhost:<port>/api/balances?as_of=banana'` — 400 with `field: "as_of"`.

## Commit shape

Single commit, scope `(api)`:

```
feat(api): GET /api/balances for bulk account-balance snapshots

Adds AccountService.GetAccountBalancesBulk(ctx, asOf, includeHidden) and a
GET /api/balances handler that returns a ListResult[AccountBalance] of all
accounts with their balances at a point in time. Defaults: as_of=now,
include_hidden=false. No pagination (account counts are bounded). Per-currency
totals are deferred to the SPA, which has every row and can aggregate as it
likes for any given view.
```

The model addition (`AccountBalance`), service method, handler, route registration, and tests all ship together — they form one cohesive endpoint contract.
