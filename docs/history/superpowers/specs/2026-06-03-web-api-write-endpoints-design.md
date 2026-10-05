# Web API Write Endpoints — Accounts and Transactions

**Date:** 2026-06-03
**Status:** Approved design — ready for implementation plan
**Scope:** Create / update / delete for accounts and transactions. Reconciliation, ledger switching, and the SPA are out of scope.

## Context

The foundation spec ([`2026-06-02-web-api-foundation-design.md`](2026-06-02-web-api-foundation-design.md)) landed the router, middleware chain, `apiHandler` adapter, error-to-status mapping, and JSON helpers. The read-endpoints spec ([`2026-06-03-web-api-read-endpoints-design.md`](2026-06-03-web-api-read-endpoints-design.md), PR #176) added the read-only domain surface and the `testhelper_test.go` substrate with a real in-memory service.

This spec adds the write surface — seven endpoints across accounts and transactions. Every endpoint maps onto an existing method on `svc.Account()` or `svc.Transaction()`. No new service methods, no new repo methods, no new error sentinels, no new dependencies. The work brings four previously-unreachable error branches online (`ErrAlreadyExists`, `ErrReconciled`, `ErrCircularParent`, `ErrNotEditable`) and pins them with integration tests.

This is step 5 from the pre-development review's implementation order ([`docs/web-layer/2026-06-02-pre-development-review.md`](../../web-layer/2026-06-02-pre-development-review.md)).

## Decisions

| Concern | Choice | Rationale |
|---------|--------|-----------|
| Transaction create shape | Splits-only (`CreateTransactionFromSplitsInput`) | One endpoint; SPA owns the from/to → splits expansion. `CreateSimpleTransaction` stays a CLI-only convenience. |
| Transaction update shape | PUT-style full replace + `/status` sub-route | Mirrors `UpdateTransactionComplete` + `UpdateTransactionStatus` 1:1. Status toggle stays cheap. |
| Account update shape | One PATCH, field-presence dispatch | `{name?, description?, is_hidden?}` — handler routes to `RenameAccount` and/or `UpdateAccountMetadata` based on which fields are present. |
| Request body source | Reuse model input structs with JSON tags added in place | Zero translation layer; matches the read spec's bare-model philosophy. One API-local DTO for account PATCH because field-presence dispatch needs pointer fields. |
| Account create with balance | `POST /api/accounts` always calls `CreateAccountWithBalance` | Service transparently degrades to `CreateAccount` when `balance == 0`. No API-level branching. |
| Account rename `name` field | Full new name (e.g. `"Assets:Bank:Checking2"`) | Matches `model.Account.Name`; SPA can round-trip the whole object. Service validates parent-path equality. |
| Path vs body authority for IDs | Path is authoritative; body `id` is forbidden | `UpdateTransactionInput.ID` becomes `json:"-"`; `DisallowUnknownFields` rejects any `"id"` in the body. No smuggling. |
| Delete response | `200 {"deleted": true, "id": N}` | Lightweight confirmation; lets the SPA log/toast without re-serializing the resource. |
| Account delete lookup | `DELETE /api/accounts/{id}`; handler resolves name, calls `DeleteAccountByName` | One extra `GetAccountByID` for the name. Avoids adding `DeleteAccountByID` to the service. |
| New deps / sentinels / service methods | None | All wiring uses existing service surface. |

## Endpoint Catalog

### Accounts

| Method | Path | Body | Returns |
|--------|------|------|---------|
| POST | `/api/accounts` | `CreateAccountInput` | `201` `*Account` |
| PATCH | `/api/accounts/{id}` | `{name?, description?, is_hidden?}` | `200` `*Account` |
| DELETE | `/api/accounts/{id}` | — | `200 {"deleted": true, "id": N}` |

### Transactions

| Method | Path | Body | Returns |
|--------|------|------|---------|
| POST | `/api/transactions` | `CreateTransactionFromSplitsInput` | `201` `*TransactionDetail` |
| PATCH | `/api/transactions/{id}` | `UpdateTransactionInput` (no `id` field) | `200` `*TransactionDetail` |
| PATCH | `/api/transactions/{id}/status` | `{"status": "Pending"\|"Cleared"}` | `200` `*TransactionDetail` |
| DELETE | `/api/transactions/{id}` | — | `200 {"deleted": true, "id": N}` |

### Mechanical notes

- `POST /api/accounts` always calls `CreateAccountWithBalance(ctx, input)`. When `input.Balance == 0`, the service skips the opening-balance transaction internally.
- `PATCH /api/accounts/{id}` with `name` present and different from current calls `RenameAccount`; with `description` or `is_hidden` present calls `UpdateAccountMetadata`. Both may run in one request (rename first, then metadata against the renamed entity). All fields absent → 400 `validation_failed` "no updatable fields".
- `PATCH /api/accounts/{id}` is **not atomic** across rename + metadata: if rename succeeds and metadata fails, the rename has already committed. This matches the service's current shape (the two operations are independent service calls). Acceptable for the local single-user setting; the SPA retries metadata against the new name and converges.
- `PATCH /api/transactions/{id}` always calls `UpdateTransactionComplete` (full replace, atomic via `ExecTx`).
- `PATCH /api/transactions/{id}/status` calls `UpdateTransactionStatus`; rejects `Reconciled` per the service contract — the reconcile lifecycle stays in its own (future) spec.
- DELETE on accounts: handler fetches by ID (gives the name + a clean 404), then calls `DeleteAccountByName`.
- All four write-only sentinels (`ErrAlreadyExists`, `ErrReconciled`, `ErrCircularParent`, `ErrNotEditable`) become exercisable; tests assert each maps to the documented status.

## Request and Response Shapes

### JSON tags added to model input structs

`internal/model/input.go` gets tags added in place. No struct shape changes.

```go
type CreateAccountInput struct {
    Name        string      `json:"name"`
    Type        AccountType `json:"type"`
    Currency    string      `json:"currency"`
    Description string      `json:"description"`
    ParentID    *int64      `json:"parent_id,omitempty"`
    Balance     int64       `json:"balance"`
}

type CreateTransactionFromSplitsInput struct {
    Splits      []SplitDetail     `json:"splits"`
    Description string            `json:"description"`
    Timestamp   int64             `json:"timestamp"`
    Status      TransactionStatus `json:"status"`
    Type        TransactionType   `json:"type"`
}

type UpdateTransactionInput struct {
    ID          int64             `json:"-"`
    Description string            `json:"description"`
    Timestamp   int64             `json:"timestamp"`
    Status      TransactionStatus `json:"status"`
    Type        TransactionType   `json:"type"`
    Splits      []SplitDetail     `json:"splits"`
}
```

`json:"-"` on `UpdateTransactionInput.ID` keeps the path authoritative. Combined with `decodeJSON`'s `DisallowUnknownFields`, a body that includes `"id"` is rejected with 400.

`SplitDetail`, `AccountType`, `TransactionStatus`, `TransactionType` are already JSON-tagged from earlier work.

### API-local DTOs

Two DTOs live in `internal/api/`:

```go
// accounts_write.go
type updateAccountRequest struct {
    Name        *string `json:"name,omitempty"`
    Description *string `json:"description,omitempty"`
    IsHidden    *bool   `json:"is_hidden,omitempty"`
}

// transactions_write.go
type updateStatusRequest struct {
    Status model.TransactionStatus `json:"status"`
}
```

Pointer fields on `updateAccountRequest` distinguish "absent" from "zero value" — required for field-presence dispatch. All three nil → 400. `Name` non-nil and equal to current → silently skip rename; other fields still apply.

### Response shapes

- `POST /api/accounts` → `201` + `*model.Account` (bare).
- `PATCH /api/accounts/{id}` → `200` + `*model.Account` after both ops have run. Handler returns the result of the last service call (renamed-then-metadata-updated entity).
- `POST /api/transactions` → `201` + `*model.TransactionDetail`. Service's `CreateTransactionFromSplits` returns the new ID but the echoed splits don't carry resolved `AccountID`/`AccountType`; handler refetches via `GetTransactionByID(id)`.
- `PATCH /api/transactions/{id}` and `/status` → `200` + `*model.TransactionDetail`. Handler refetches after `UpdateTransactionComplete` / `UpdateTransactionStatus`, both of which return only `error`.
- All DELETEs → `200 {"deleted": true, "id": N}`.

### Error body (already in foundation)

```json
{ "error": "validation_failed", "message": "...", "field": "..." }
```

`field` is omitted when empty (the `omitempty` tag on `errorBody.Field`).

## Handler Structure

Three new handler files; methods on `*Server` matching the read-spec convention.

### `internal/api/accounts_write.go`

```go
func (s *Server) handleCreateAccount(w http.ResponseWriter, r *http.Request) error {
    var input model.CreateAccountInput
    if err := decodeJSON(r, &input); err != nil { return err }
    acc, err := s.svc.Account().CreateAccountWithBalance(r.Context(), input)
    if err != nil { return err }
    return writeJSON(w, 201, acc)
}

func (s *Server) handleUpdateAccount(w http.ResponseWriter, r *http.Request) error {
    id, err := parseInt64Path(r, "id")
    if err != nil { return err }
    var req updateAccountRequest
    if err := decodeJSON(r, &req); err != nil { return err }
    if req.Name == nil && req.Description == nil && req.IsHidden == nil {
        return &service.ValidationError{Message: "no updatable fields provided"}
    }
    ctx := r.Context()

    current, err := s.svc.Account().GetAccountByID(ctx, id)
    if err != nil { return err }

    updated := current
    if req.Name != nil && *req.Name != current.Name {
        renamed, err := s.svc.Account().RenameAccount(ctx, current.Name, *req.Name)
        if err != nil { return err }
        updated = renamed
    }
    if req.Description != nil || req.IsHidden != nil {
        desc, hidden := updated.Description, updated.IsHidden
        if req.Description != nil { desc = *req.Description }
        if req.IsHidden != nil    { hidden = *req.IsHidden }
        meta, err := s.svc.Account().UpdateAccountMetadata(ctx, updated.ID, desc, hidden)
        if err != nil { return err }
        updated = meta
    }
    return writeJSON(w, 200, updated)
}

func (s *Server) handleDeleteAccount(w http.ResponseWriter, r *http.Request) error {
    id, err := parseInt64Path(r, "id")
    if err != nil { return err }
    ctx := r.Context()
    acc, err := s.svc.Account().GetAccountByID(ctx, id)
    if err != nil { return err }
    if err := s.svc.Account().DeleteAccountByName(ctx, acc.Name); err != nil {
        return err
    }
    return writeJSON(w, 200, map[string]any{"deleted": true, "id": id})
}
```

### `internal/api/transactions_write.go`

```go
func (s *Server) handleCreateTransaction(w http.ResponseWriter, r *http.Request) error {
    var input model.CreateTransactionFromSplitsInput
    if err := decodeJSON(r, &input); err != nil { return err }
    ctx := r.Context()
    detail, err := s.svc.Transaction().CreateTransactionFromSplits(ctx, input)
    if err != nil { return err }
    full, err := s.svc.Transaction().GetTransactionByID(ctx, detail.ID)
    if err != nil { return err }
    return writeJSON(w, 201, full)
}

func (s *Server) handleUpdateTransaction(w http.ResponseWriter, r *http.Request) error {
    id, err := parseInt64Path(r, "id")
    if err != nil { return err }
    var input model.UpdateTransactionInput
    if err := decodeJSON(r, &input); err != nil { return err }
    input.ID = id
    ctx := r.Context()
    if err := s.svc.Transaction().UpdateTransactionComplete(ctx, input); err != nil {
        return err
    }
    detail, err := s.svc.Transaction().GetTransactionByID(ctx, id)
    if err != nil { return err }
    return writeJSON(w, 200, detail)
}

func (s *Server) handleUpdateTransactionStatus(w http.ResponseWriter, r *http.Request) error {
    id, err := parseInt64Path(r, "id")
    if err != nil { return err }
    var req updateStatusRequest
    if err := decodeJSON(r, &req); err != nil { return err }
    ctx := r.Context()
    if err := s.svc.Transaction().UpdateTransactionStatus(ctx, id, req.Status); err != nil {
        return err
    }
    detail, err := s.svc.Transaction().GetTransactionByID(ctx, id)
    if err != nil { return err }
    return writeJSON(w, 200, detail)
}

func (s *Server) handleDeleteTransaction(w http.ResponseWriter, r *http.Request) error {
    id, err := parseInt64Path(r, "id")
    if err != nil { return err }
    if err := s.svc.Transaction().DeleteTransaction(r.Context(), id); err != nil {
        return err
    }
    return writeJSON(w, 200, map[string]any{"deleted": true, "id": id})
}
```

### Router additions — `internal/api/router.go`

```go
r.Method("POST",   "/accounts",                 apiHandler(s.handleCreateAccount))
r.Method("PATCH",  "/accounts/{id}",            apiHandler(s.handleUpdateAccount))
r.Method("DELETE", "/accounts/{id}",            apiHandler(s.handleDeleteAccount))

r.Method("POST",   "/transactions",             apiHandler(s.handleCreateTransaction))
r.Method("PATCH",  "/transactions/{id}",        apiHandler(s.handleUpdateTransaction))
r.Method("PATCH",  "/transactions/{id}/status", apiHandler(s.handleUpdateTransactionStatus))
r.Method("DELETE", "/transactions/{id}",        apiHandler(s.handleDeleteTransaction))
```

## Error Mapping

No changes to `mapError`. The foundation covers every sentinel this spec touches. What this work does is make four branches *reachable* for the first time.

| Sentinel | Status | First exercised by |
|----------|--------|--------------------|
| `*ValidationError` | 400 | Many new paths: bad split, balance ≠ 0, unknown type, empty description, duplicate or foreign split IDs, rename across parent path, "no updatable fields". |
| `ErrNotFound` | 404 | PATCH/DELETE against unknown IDs. |
| `ErrAlreadyExists` | 409 | `POST /api/accounts` with a duplicate name. (Transaction `external_id` is not in `CreateTransactionFromSplitsInput`, so the duplicate-`external_id` path is not reachable from this surface.) |
| `ErrReconciled` | 409 | PATCH (full or status) and DELETE against a reconciled transaction. |
| `ErrCircularParent` | 409 | `POST /api/accounts` with a `parent_id` whose chain cycles. |
| `ErrNotEditable` | 403 | DELETE or rename of `Equity:OpeningBalances_*`. |

### Decode failures

`decodeJSON` calls `DisallowUnknownFields()` and wraps decode errors as `*service.ValidationError{Field: "body"}` → 400:

- Sending `{"unknown_field": ...}` → 400.
- Sending `"id"` in a PATCH body for transactions → 400 (because `UpdateTransactionInput.ID` is `json:"-"`).
- Empty body to PATCH/POST → 400.

### Validation field names

The service-layer `*ValidationError` carries user-meaningful `Field` values that match request JSON field names (`"name"`, `"description"`, `"currency"`, `"type"`, `"parent"`, `"splits"`, `"status"`, `"memo"`, `"amount"`, `"account"`). Cross-field validations from `DeleteAccountByName` ("has children", "has transactions") use empty `Field`; the SPA reads `message` for the user-facing string.

### Known rough edge — `repository.ErrNotFound` in split account lookup *(Resolved)*

Resolved in [`2026-06-05-fix-create-transaction-split-account-errnotfound-design.md`](2026-06-05-fix-create-transaction-split-account-errnotfound-design.md). The service layer now translates `repository.ErrNotFound` from `GetAccountByName` into `validationErrorf("splits", ...)`, yielding 400 with `error="validation_failed"` and `field="splits"` — matching `UpdateTransaction`'s existing behavior.

## Testing

All tests stay in `internal/api/`, table-driven, `httptest` + stdlib only. The read spec already built `internal/api/testhelper_test.go` with `newTestServer(t) → (*httptest.Server, *service.Service)` over a real in-memory SQLite — we reuse it. No new helpers needed beyond ad-hoc seed functions per test.

### `internal/api/accounts_write_test.go`

**`handleCreateAccount`**
- Asset account, no balance → 201 with `id`, `name`, `type`, `currency`; `parent_id` omitted.
- Asset account, `balance: 100000` → 201; follow-up `/api/accounts/{id}/balance` returns 100000; `Equity:OpeningBalances_USD` was auto-created.
- Liability account with balance → balance check confirms opening split direction (sign reversed vs Asset).
- Duplicate name → 409, `error: "already_exists"`. **Exercises `ErrAlreadyExists`.**
- Invalid type (`"Z"`) → 400, `field: "type"`.
- Empty name → 400, `field: "name"`.
- Description > 500 chars → 400, `field: "description"`.
- `parent_id` forming a cycle → 409, `error: "circular_parent"`. **Exercises `ErrCircularParent`.** Set up the cycle via direct repo writes in test seed (two accounts with mutually-pointing `parent_id`), then attempt to create a child.
- Unknown JSON field → 400.

**`handleUpdateAccount`**
- All three fields nil → 400 "no updatable fields provided".
- `name` only, valid same-prefix new name → 200; subsequent GET by old name → 404, by new name → 200.
- `description` and `is_hidden` together → 200 with both fields reflected.
- `name` + `description` in one request → 200 reflects both; metadata landed on the renamed entity.
- `name` equal to current → no-op rename; other fields still apply.
- `name` with different parent path → 400, `field: "name"` ("rename cannot change parent path").
- Rename of `Equity:OpeningBalances_USD` → 403, `error: "not_editable"`. **Exercises `ErrNotEditable`** (spot 1 of 2).
- Non-existent ID → 404.

**`handleDeleteAccount`**
- Empty leaf → 200 `{"deleted": true, "id": N}`; subsequent GET → 404.
- Account with children → 400, `validation_failed`, empty `field`.
- Account with transactions → 400.
- `Equity:OpeningBalances_USD` → 403, `error: "not_editable"`. **Exercises `ErrNotEditable`** (spot 2 of 2).
- Non-existent ID → 404.

### `internal/api/transactions_write_test.go`

**`handleCreateTransaction`**
- Balanced 2-split Expense between `Assets:Bank:Checking` and `Expenses:Coffee` → 201 with full `TransactionDetail`; both splits carry `account_id`, `account_name`, `account_type`.
- Unbalanced splits → 400, `field: "splits"`.
- 1-split body → 400, `field: "splits"`.
- Type label not matching splits → 400, `field: "splits"`.
- Status `"Reconciled"` on create → 400, `field: "status"`.
- Empty description → 400, `field: "description"`.
- Split referencing a nonexistent account → returns 400 with `field: "splits"` (resolved by `2026-06-05-fix-create-transaction-split-account-errnotfound-design.md`).
- Split memo > 200 chars → 400, `field: "memo"`.
- Hidden account in a split → 400, `field: "account"`.
- Parent (non-leaf) account in a split → 400, `field: "account"`.

**`handleUpdateTransaction`**
- Full replace of a Cleared transaction's description + splits → 200, body reflects.
- Update against a Reconciled tx → 409, `error: "reconciled"`. **Exercises `ErrReconciled`** (spot 1 of 3). Seed: create transaction, then mark splits + tx reconciled via repo (`MarkSplitsReconciledByAccount` + `BulkUpdateTransactionStatus`).
- Body with `"id"` field → 400 (unknown field).
- Non-existent ID → 404.
- Unbalanced splits → 400.
- Split ID belonging to a different transaction → 400, `field: "splits"`.

**`handleUpdateTransactionStatus`**
- Pending → Cleared → 200 with new status.
- Cleared → Pending → 200.
- `"Reconciled"` → 400, `field: "status"`.
- Status change on Reconciled tx → 409. **Exercises `ErrReconciled`** (spot 2 of 3).
- Non-existent ID → 404.

**`handleDeleteTransaction`**
- Cleared → 200 `{"deleted": true, "id": N}`; subsequent GET → 404.
- Pending → 200.
- Reconciled → 409. **Exercises `ErrReconciled`** (spot 3 of 3).
- Non-existent ID → 404.

### `internal/model/json_test.go` additions

Round-trip cases for the newly tagged input structs:

- `CreateAccountInput` — marshals with snake_case `parent_id`, omits when nil; unmarshals from the same.
- `CreateTransactionFromSplitsInput` — round-trip across `splits`, `description`, `timestamp`, `status`, `type`.
- `UpdateTransactionInput` — `id` field invisible to encode/decode; body without `id` unmarshals cleanly.

### What earlier specs' tests already cover (not re-tested)

- Error-to-status mapping per sentinel — `errors_test.go`.
- `apiHandler`, `writeJSON`, `decodeJSON`, `DisallowUnknownFields` — covered.
- Middleware, request ID, access log, CORS, graceful shutdown — covered.
- Read endpoints — unchanged.

## Files Touched

**Added:**
- `internal/api/accounts_write.go`
- `internal/api/transactions_write.go`
- `internal/api/accounts_write_test.go`
- `internal/api/transactions_write_test.go`

**Modified:**
- `internal/model/input.go` — JSON tags on `CreateAccountInput`, `CreateTransactionFromSplitsInput`, `UpdateTransactionInput`.
- `internal/api/router.go` — register seven new routes.
- `internal/model/json_test.go` — round-trip cases for the now-tagged input structs.

**Unchanged:**
- `internal/api/handler.go`, `errors.go`, `middleware.go`, `health.go`, `params.go`, `accounts.go`, `transactions.go`, `reports.go`, `server.go` and their tests.
- Service layer, repo, store, config, migrations. No new service methods, no new repo methods, no new sentinels, no new dependencies.

## Out of Scope

- Reconcile endpoints (unreconciled list, preview, commit) — separate spec.
- Bulk operations (multi-create, multi-delete).
- `external_id` on transactions — not in `CreateTransactionFromSplitsInput`.
- Ledger management endpoints; the #119 split-brain limitation stays out.
- Optimistic concurrency / `If-Match` / version fields — single-user local assumption holds.
- ~~Fix for `repository.ErrNotFound` vs `service.ErrNotFound` mismatch in split account resolution~~ — resolved in `2026-06-05-fix-create-transaction-split-account-errnotfound-design.md`.
- The React SPA.
- Authentication, TLS, metrics, tracing, rate limiting — same as in earlier specs.
