# Web API Reconciliation Endpoints

**Date:** 2026-06-04
**Status:** Approved design — ready for implementation plan
**Scope:** REST endpoints for the reconciliation workflow — list unreconciled, preview, commit. The React SPA, an unreconcile/undo path, and reconciliation history are out of scope.

## Context

The foundation spec ([`2026-06-02-web-api-foundation-design.md`](2026-06-02-web-api-foundation-design.md)), read-endpoints spec ([`2026-06-03-web-api-read-endpoints-design.md`](2026-06-03-web-api-read-endpoints-design.md)), write-endpoints spec ([`2026-06-03-web-api-write-endpoints-design.md`](2026-06-03-web-api-write-endpoints-design.md)), and ledgers spec ([`2026-06-04-web-api-ledgers-design.md`](2026-06-04-web-api-ledgers-design.md)) landed the foundation plus 24 endpoints across accounts, transactions, reports, and ledger management.

This spec adds the reconciliation surface — three endpoints that let the SPA load an account's unreconciled transactions, preview the balance delta for a candidate selection, and commit the reconciliation. Every endpoint maps onto an existing method on `svc.Transaction()`: `GetUnreconciledByAccount`, `PreviewReconcile`, `ReconcileTransactions`. No new service methods, no new repo methods, no new dependencies. The work brings the reconciliation workflow online over HTTP and adds one API-local error type for the balance-mismatch policy.

This is step 6 from the pre-development review's implementation order ([`docs/web-layer/2026-06-02-pre-development-review.md`](../../web-layer/2026-06-02-pre-development-review.md)).

## Decisions

| Concern | Choice | Rationale |
|---------|--------|-----------|
| Endpoint shape | Three endpoints: list, preview, commit | Matches the service-layer split 1:1; mirrors the CLI's non-interactive preview-then-commit flow; lets the SPA defer the "are you sure?" check to a server-authoritative endpoint. |
| Mismatch policy | Server-side `allow_mismatch` flag (default `false`) | Mirrors `--force` from the CLI. Commit handler runs `PreviewReconcile` first when the flag is off; rejects with 409 if diff ≠ 0. Defense-in-depth against SPA bugs without a service-layer change. |
| Mismatch error transport | API-local `*balanceMismatchError` + extended `errorBody.Difference` | The mismatch concept is API-layer policy (the CLI handles `--force` in CLI code, the service has no opinion). Keep it where it belongs; the response envelope grows one optional field. |
| Account ID location | Path-only on all three endpoints | Matches the existing convention from accounts/transactions write endpoints. |
| `last_reconciled_balance` on commit | Re-derive: `statement_balance - difference` | Algebraic identity from the service contract. One subtraction in the handler; no new service method, no extra DB call. |
| Idempotency | Implicit via service validation | After a successful commit, replayed IDs are no longer in the unreconciled set → service returns 400 with a clear message. No explicit idempotency key — overengineering for local single-user. |
| Account-existence check on `GET /unreconciled` | Handler does an explicit `GetAccountByID` | The service method returns an empty list for unknown IDs; the API contract is 404. Handler adds the check. |
| New deps / sentinels / service methods | None | All wiring uses the existing service surface and the foundation error machinery. |

## Endpoint Catalog

| Method | Path | Body / Param | Returns |
|--------|------|--------------|---------|
| GET | `/api/accounts/{id}/unreconciled` | — | `200 {entries: [ReconcileEntry], last_reconciled_balance: int64}` |
| POST | `/api/accounts/{id}/reconcile/preview` | `{statement_balance, transaction_ids}` | `200 {difference: int64}` |
| POST | `/api/accounts/{id}/reconcile` | `{statement_balance, transaction_ids, allow_mismatch?}` | `200 {reconciled_count, difference, last_reconciled_balance}` or `409 {error: "balance_mismatch", difference}` |

### Mechanical notes

- `statement_balance` is `int64` cents — same convention as every other amount on the API.
- `transaction_ids` is `[]int64`. Empty → 400 (service-layer validation).
- `allow_mismatch` defaults to `false`. When the flag is off, the commit handler calls `PreviewReconcile` first; only proceeds to `ReconcileTransactions` if `diff == 0`. When `true`, it skips straight to the atomic commit (the service still validates IDs and computes the diff inside the transaction; see #127 TOCTOU fix).
- Chi route registration: `/reconcile/preview` is registered before `/reconcile` so the longer literal path wins for that subtree.
- `GET /api/accounts/{id}/unreconciled` calls `GetAccountByID` first to surface 404 for unknown account IDs. The underlying service method does not check existence (returns an empty list), so the API-layer check restores consistency with the rest of the surface.
- The list endpoint returns both Pending and Cleared transactions — that matches the service contract (the reconciliation workflow operates on the union).

## Request and Response Shapes

### API-local DTOs — `internal/api/reconcile.go`

```go
type unreconciledResponse struct {
    Entries               []*model.ReconcileEntry `json:"entries"`
    LastReconciledBalance int64                   `json:"last_reconciled_balance"`
}

type reconcilePreviewRequest struct {
    StatementBalance int64   `json:"statement_balance"`
    TransactionIDs   []int64 `json:"transaction_ids"`
}

type reconcilePreviewResponse struct {
    Difference int64 `json:"difference"`
}

type reconcileCommitRequest struct {
    StatementBalance int64   `json:"statement_balance"`
    TransactionIDs   []int64 `json:"transaction_ids"`
    AllowMismatch    bool    `json:"allow_mismatch"`
}

type reconcileCommitResponse struct {
    ReconciledCount       int   `json:"reconciled_count"`
    Difference            int64 `json:"difference"`
    LastReconciledBalance int64 `json:"last_reconciled_balance"`
}
```

`reconcilePreviewRequest` and `reconcileCommitRequest` are distinct types so each endpoint can grow independently — same pattern the ledgers spec used for create/switch.

`AllowMismatch` is `bool`, not `*bool`. Absent and explicit `false` are semantically identical: enforce the gate.

`model.ReconcileEntry` is already JSON-tagged (`id`, `timestamp`, `description`, `status`, `amount`, `offset_account`) and has an existing round-trip test in `internal/model/json_test.go`. No model changes needed.

### Response examples

```json
GET /api/accounts/42/unreconciled
{
  "entries": [
    {"id": 18, "timestamp": 1735689600, "description": "Coffee", "status": "Cleared", "amount": -450, "offset_account": "Expenses:Coffee"},
    {"id": 19, "timestamp": 1735776000, "description": "Salary", "status": "Cleared", "amount": 500000, "offset_account": "Income:Salary"}
  ],
  "last_reconciled_balance": 250000
}

POST /api/accounts/42/reconcile/preview
body: {"statement_balance": 749550, "transaction_ids": [18, 19]}
→ 200 {"difference": 0}

POST /api/accounts/42/reconcile
body: {"statement_balance": 749550, "transaction_ids": [18, 19]}
→ 200 {"reconciled_count": 2, "difference": 0, "last_reconciled_balance": 749550}

POST /api/accounts/42/reconcile (mismatch, no flag)
body: {"statement_balance": 800000, "transaction_ids": [18, 19]}
→ 409 {"error": "balance_mismatch", "message": "statement balance off by 50450", "difference": 50450}

POST /api/accounts/42/reconcile (mismatch + acknowledged)
body: {"statement_balance": 800000, "transaction_ids": [18, 19], "allow_mismatch": true}
→ 200 {"reconciled_count": 2, "difference": 50450, "last_reconciled_balance": 749550}
```

### Error body (extended)

```go
type errorBody struct {
    Error      string `json:"error"`
    Message    string `json:"message"`
    Field      string `json:"field,omitempty"`
    Difference *int64 `json:"difference,omitempty"` // new — only populated on balance_mismatch
}
```

`Difference` is a pointer so the field is omitted on all other error responses; the existing error contracts are unchanged.

## Error Mapping

### New error type — `*balanceMismatchError`

Lives in `internal/api/errors.go`:

```go
// balanceMismatchError carries the diff so the SPA can render "off by $X"
// without re-previewing. Mirrors --force semantics from the CLI.
type balanceMismatchError struct {
    Difference int64
}

func (e *balanceMismatchError) Error() string {
    return fmt.Sprintf("statement balance off by %d", e.Difference)
}
```

One `errors.As` branch in `mapError`:

```go
var bme *balanceMismatchError
if errors.As(err, &bme) {
    diff := bme.Difference
    return 409, errorBody{
        Error:      "balance_mismatch",
        Message:    bme.Error(),
        Difference: &diff,
    }
}
```

### Error code table

| Sentinel | Status | Error code | First exercised by |
|----------|--------|------------|--------------------|
| `*balanceMismatchError` (new, API-local) | 409 | `balance_mismatch` | Commit endpoint with `allow_mismatch: false` and non-zero diff. |
| `*service.ValidationError` | 400 | `validation_failed` | Empty `transaction_ids`, duplicate IDs, ID not in unreconciled set, malformed body. From `PreviewReconcile` / `ReconcileTransactions`. |
| `service.ErrNotFound` | 404 | `not_found` | Unknown account ID (handler check on GET; service check on preview/commit). |

### Decode failures

Standard `decodeJSON` with `DisallowUnknownFields()` — unknown fields, malformed JSON, or empty body → 400 with `field: "body"`. Same machinery as every other write endpoint.

## Handler Structure

### `internal/api/reconcile.go`

Three handlers as methods on `*Server`. Each ≤ 25 lines.

```go
func (s *Server) handleListUnreconciled(w http.ResponseWriter, r *http.Request) error {
    id, err := parseInt64Path(r, "id")
    if err != nil { return err }
    ctx := r.Context()
    if _, err := s.svc.Account().GetAccountByID(ctx, id); err != nil {
        return err
    }
    entries, lastBalance, err := s.svc.Transaction().GetUnreconciledByAccount(ctx, id)
    if err != nil { return err }
    return writeJSON(w, 200, unreconciledResponse{
        Entries:               entries,
        LastReconciledBalance: lastBalance,
    })
}

func (s *Server) handleReconcilePreview(w http.ResponseWriter, r *http.Request) error {
    id, err := parseInt64Path(r, "id")
    if err != nil { return err }
    var req reconcilePreviewRequest
    if err := decodeJSON(r, &req); err != nil { return err }
    diff, err := s.svc.Transaction().PreviewReconcile(r.Context(), id, req.StatementBalance, req.TransactionIDs)
    if err != nil { return err }
    return writeJSON(w, 200, reconcilePreviewResponse{Difference: diff})
}

func (s *Server) handleReconcileCommit(w http.ResponseWriter, r *http.Request) error {
    id, err := parseInt64Path(r, "id")
    if err != nil { return err }
    var req reconcileCommitRequest
    if err := decodeJSON(r, &req); err != nil { return err }
    ctx := r.Context()

    if !req.AllowMismatch {
        diff, err := s.svc.Transaction().PreviewReconcile(ctx, id, req.StatementBalance, req.TransactionIDs)
        if err != nil { return err }
        if diff != 0 {
            return &balanceMismatchError{Difference: diff}
        }
    }

    diff, err := s.svc.Transaction().ReconcileTransactions(ctx, id, req.StatementBalance, req.TransactionIDs)
    if err != nil { return err }

    return writeJSON(w, 200, reconcileCommitResponse{
        ReconciledCount:       len(req.TransactionIDs),
        Difference:            diff,
        LastReconciledBalance: req.StatementBalance - diff,
    })
}
```

### Notes on the gate ordering

- `PreviewReconcile` runs first when the gate is on. Preview is read-only; if validation fails (bad IDs, duplicates, empty list), the error surfaces before any write.
- `allow_mismatch: true` skips preview entirely. `ReconcileTransactions` runs its own validation inside `ExecTx` (the TOCTOU fix from #127), so this is not bypassing safety — only skipping the redundant read.
- The double-read window between preview and commit is acceptable. Between them, the unreconciled set could theoretically change (e.g., CLI running concurrently). The commit-time validation catches this and returns 400. Single-user local; not a real concern.

### `last_reconciled_balance` re-derivation

From the service contract:

```
diff = statementBalance − (lastBalance + clearedBalance)
new_last = lastBalance + clearedBalance = statementBalance − diff
```

One subtraction in the handler. No new service method, no extra DB call.

### Router additions — `internal/api/router.go`

```go
r.Method("GET",  "/accounts/{id}/unreconciled",       apiHandler(s.handleListUnreconciled))
r.Method("POST", "/accounts/{id}/reconcile/preview",  apiHandler(s.handleReconcilePreview))
r.Method("POST", "/accounts/{id}/reconcile",          apiHandler(s.handleReconcileCommit))
```

## Testing

All tests in `internal/api/reconcile_test.go`, table-driven, `httptest` + stdlib only. Reuses the existing `newTestServer` from the read-spec substrate (real in-memory SQLite service). No new helpers beyond inline `seedReconcileFixture(t, svc)` factory functions per test.

### `handleListUnreconciled`

- Account with no transactions → `200 {entries: [], last_reconciled_balance: 0}`.
- Account with three Cleared transactions, none reconciled → `200`; `entries` has length 3; each entry's `amount` and `offset_account` populated correctly; `last_reconciled_balance: 0`.
- Account where one prior transaction was reconciled (with `SetLastReconciledBalance` to 250000 via repo seed) → `entries` excludes the reconciled one; `last_reconciled_balance: 250000`.
- Pending transactions appear in `entries` (service includes both Pending and Cleared).
- Nonexistent account ID → `404 {error: "not_found"}`. **Verifies the handler's explicit existence check.**
- Non-numeric path param → `400`.

### `handleReconcilePreview`

- Selection that exactly bridges the gap (`statement_balance == lastBalance + sum(selected.amount)`) → `200 {difference: 0}`.
- Selection that under-shoots by 5000 → `200 {difference: 5000}`.
- Selection that over-shoots → `200` with negative `difference`.
- Empty `transaction_ids` → `400`, `field: "transactions"`.
- Duplicate ID → `400`, `field: "transactions"`.
- ID not in unreconciled set (already reconciled, or belongs to a different account) → `400`, `field: "transactions"`.
- Nonexistent account ID → `404` (service's `GetAccountByID` failure).
- Unknown JSON field → `400`.
- Empty body → `400`.
- **Preview does not write.** Follow-up `GET /unreconciled` still shows the same entries.

### `handleReconcileCommit`

**Happy paths:**
- Clean reconcile (diff == 0, `allow_mismatch` omitted) → `200 {reconciled_count: 2, difference: 0, last_reconciled_balance: <statementBalance>}`; follow-up `GET /unreconciled` excludes the committed IDs; `GetLastReconciledBalance` (via repo) equals the response value.
- `allow_mismatch: true` with non-zero diff → `200`; `last_reconciled_balance` equals `statement_balance - diff` (re-derivation pinned); follow-up state reflects the commit.

**Gate paths:**
- Non-zero diff, `allow_mismatch` absent → `409 {error: "balance_mismatch", message: "...", difference: <N>}`. **Assert no write happened** — follow-up `GET /unreconciled` still includes the IDs; `GetLastReconciledBalance` unchanged.
- Non-zero diff, `allow_mismatch: false` (explicit) → same as absent → `409`.

**Validation pass-through** (exercises preview-first when gate is on, and the service's atomic validation when gate is off):
- Empty `transaction_ids`, gate on → `400` (from preview).
- Empty `transaction_ids`, `allow_mismatch: true` → `400` (from `ReconcileTransactions`).
- Duplicate IDs, gate on → `400` from preview.
- Duplicate IDs, gate off → `400` from `ReconcileTransactions`.
- ID not in unreconciled set, either mode → `400`.
- Nonexistent account ID, either mode → `404`.
- Unknown body field → `400`.

**Replay safety:**
- Commit succeeds; replay the same body → `400` (IDs no longer in unreconciled set). Pins the "natural idempotency via validation" property.

### `errors_test.go` addition

Table row for `*balanceMismatchError` → `409`, `error: "balance_mismatch"`, `difference` field populated in the JSON body. One wrapped case (`fmt.Errorf("...: %w", &balanceMismatchError{Difference: 100})`) to verify `errors.As` traversal.

### `model/json_test.go` — no changes

`ReconcileEntry` is already JSON-tagged with an existing round-trip test.

### What earlier specs already cover (not re-tested)

- `apiHandler`, `writeJSON`, `decodeJSON`, `DisallowUnknownFields` — foundation.
- Sentinel-to-status mapping infrastructure — `errors_test.go` (this spec adds one row, not new infrastructure).
- Middleware, request ID, access log, CORS, graceful shutdown — covered.
- Service-layer reconcile logic — `internal/service/reconcile_ops_test.go`.

## Files Touched

**Added:**
- `internal/api/reconcile.go` — three handlers + four DTOs + `balanceMismatchError` type.
- `internal/api/reconcile_test.go` — table-driven coverage.

**Modified:**
- `internal/api/router.go` — register three new routes.
- `internal/api/errors.go` — one `errors.As` branch for `*balanceMismatchError`; extend `errorBody` with optional `Difference *int64`.
- `internal/api/errors_test.go` — table rows for the new error type (direct + wrapped).

**Unchanged:**
- Service layer, repository, store, model, migrations, config struct.
- Existing API handler files (`accounts.go`, `transactions.go`, `reports.go`, `accounts_write.go`, `transactions_write.go`, `ledgers.go`, `params.go`, `handler.go`, `middleware.go`, `health.go`, `server.go`) and their tests.
- CLI `cmd/reconcile*.go` — continues to operate on the service layer directly with `--force` semantics intact.

## Out of Scope

- **Atomic preview-then-commit at the service layer.** The handler does two service calls when the gate is on. An atomic `ReconcileTransactionsStrict(allowMismatch bool)` would be cleaner but requires a service-layer addition; single-user local makes the two-call window academic.
- **Unreconcile / undo.** No `DELETE /api/accounts/{id}/reconcile` to mark splits unreconciled. Service doesn't expose this; out of scope here.
- **Bulk reconcile across multiple accounts.** One account per request.
- **Reconciliation history.** No endpoint to list past reconciliation sessions or the balance at the time. Service doesn't track session metadata; out of scope.
- **`internal/ledger/registry.go` mutex race** — pre-existing data race flagged in the kickoff message; its own spec.
- **`repository.ErrNotFound` mismatch in `CreateTransaction` split account lookup** — one-line fix flagged in the write-endpoints spec; its own spec.
- **`GET /api/balances?as_of=` bulk balances** — deferred from the read-endpoints spec; its own spec.
- **Optimistic concurrency / `If-Match` headers** — single-user local assumption holds.
- **The React SPA itself.**
- **Authentication, TLS, metrics, tracing, rate limiting** — same posture as earlier specs.
