# Web API Reconciliation Endpoints Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add three HTTP endpoints (list unreconciled, preview reconcile, commit reconcile) on top of the existing `*TransactionService` reconciliation methods, with a server-side `allow_mismatch` gate that mirrors the CLI's `--force` semantics.

**Architecture:** Each handler maps 1:1 onto an existing service method (`GetUnreconciledByAccount`, `PreviewReconcile`, `ReconcileTransactions`). The commit handler enforces the mismatch gate by calling preview first when the flag is off, then proceeding to the atomic write only on `diff == 0`. The mismatch policy is API-layer only — no service changes. One API-local error type (`*balanceMismatchError`) carries the diff to the SPA via an extended `errorBody.Difference *int64` field.

**Tech Stack:** Go 1.22+, chi/v5 router, `net/http`, `database/sql` over SQLite, stdlib `testing` + `httptest`. Reuses the existing `internal/api/` substrate from the foundation, read, write, and ledger specs.

**Spec:** [`docs/superpowers/specs/2026-06-04-web-api-reconcile-design.md`](../specs/2026-06-04-web-api-reconcile-design.md)

---

## File Structure

| File | Status | Responsibility |
|------|--------|----------------|
| `internal/api/reconcile.go` | Create | Three handlers + four request/response DTOs |
| `internal/api/reconcile_test.go` | Create | Table-driven coverage of all three endpoints |
| `internal/api/errors.go` | Modify | Add `balanceMismatchError` type; extend `errorBody` with `Difference *int64`; add `errors.As` branch in `mapError` |
| `internal/api/errors_test.go` | Modify | Add a `TestMapError_BalanceMismatch` function (direct + wrapped) |
| `internal/api/router.go` | Modify | Register three new routes |

Each handler is ≤ 25 lines. The reconcile file stays focused on the reconciliation surface; no shared helpers spill into other files.

---

## Conventions (from CLAUDE.md and prior specs)

- **SPDX header on every new `.go` file:**
  ```go
  // SPDX-License-Identifier: GPL-3.0-or-later
  // Copyright (C) 2026  Hance Chin
  ```
- **Conventional commit format**, scope `(api)`. Example: `feat(api): GET /api/accounts/{id}/unreconciled`.
- **All amounts are `int64` cents.**
- **Tests:** stdlib + `httptest` only, table-driven where it pays for itself.
- **`go test ./...` and `go build ./...` must stay green** after every commit.
- **No new dependencies, no new service methods, no new repo methods, no new model fields.**

The existing test helpers in `internal/api/testhelper_test.go` are reused as-is:
- `newServerForWrite(t) → (*httptest.Server, *service.Service, *store.Store)` builds a server backed by an in-memory SQLite service.
- `seedAccount(t, svc, name, type, balance)` creates a leaf account.
- `seedTransaction(t, svc, from, to, amount, ts, desc, type, status)` creates a balanced 2-split transaction.
- `seedReconciledTransaction(t, st, accountID, txID)` flips a transaction to Reconciled by directly invoking the reconcile-aware repo methods.

The HTTP helpers `getJSON`, `postJSON`, `deleteURL` already exist in `internal/api/ledgers_test.go` (package-level, so reusable from `reconcile_test.go`).

---

## Task 1: Add `*balanceMismatchError` type and extend `errorBody`

**Files:**
- Modify: `internal/api/errors.go`
- Modify: `internal/api/errors_test.go`

This task wires up the API-local error type that the commit handler will return on a non-zero diff with `allow_mismatch: false`. Independent of the handlers — lands first so subsequent handler tests can assert against it.

- [ ] **Step 1: Write the failing test**

Append to `internal/api/errors_test.go`:

```go
func TestMapError_BalanceMismatch(t *testing.T) {
    cases := []struct {
        name           string
        err            error
        wantStatus     int
        wantCode       string
        wantDifference int64
    }{
        {"direct", &balanceMismatchError{Difference: 50450}, 409, "balance_mismatch", 50450},
        {"wrapped", fmt.Errorf("commit: %w", &balanceMismatchError{Difference: -100}), 409, "balance_mismatch", -100},
    }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            status, body := mapError(tc.err)
            if status != tc.wantStatus {
                t.Errorf("status: got %d, want %d", status, tc.wantStatus)
            }
            if body.Error != tc.wantCode {
                t.Errorf("error code: got %q, want %q", body.Error, tc.wantCode)
            }
            if body.Difference == nil {
                t.Fatalf("Difference: got nil, want pointer to %d", tc.wantDifference)
            }
            if *body.Difference != tc.wantDifference {
                t.Errorf("Difference: got %d, want %d", *body.Difference, tc.wantDifference)
            }
        })
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/api/ -run TestMapError_BalanceMismatch -v`
Expected: build failure — `undefined: balanceMismatchError` and `body.Difference undefined`.

- [ ] **Step 3: Add `balanceMismatchError` and extend `errorBody`**

In `internal/api/errors.go`:

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import (
    "errors"
    "fmt"
    "net/http"

    "github.com/hance08/kea/internal/ledger"
    "github.com/hance08/kea/internal/service"
)

type errorBody struct {
    Error      string `json:"error"`
    Message    string `json:"message"`
    Field      string `json:"field,omitempty"`
    Difference *int64 `json:"difference,omitempty"`
}

// balanceMismatchError is returned by the reconcile commit handler when the
// caller did not set allow_mismatch=true and the computed diff is non-zero.
// Mirrors --force semantics from the CLI; lives in the API layer because the
// service contract always persists regardless of diff.
type balanceMismatchError struct {
    Difference int64
}

func (e *balanceMismatchError) Error() string {
    return fmt.Sprintf("statement balance off by %d", e.Difference)
}
```

Add one branch to `mapError` immediately above the `default:` case:

```go
    var bme *balanceMismatchError
    if errors.As(err, &bme) {
        diff := bme.Difference
        return http.StatusConflict, errorBody{
            Error:      "balance_mismatch",
            Message:    bme.Error(),
            Difference: &diff,
        }
    }
```

Note: `errors.As` is used inside a `switch` that only has `case` arms for direct sentinels — restructure carefully. The final `mapError` body is:

```go
func mapError(err error) (int, errorBody) {
    var verr *service.ValidationError
    if errors.As(err, &verr) {
        return http.StatusBadRequest, errorBody{
            Error: "validation_failed", Message: verr.Message, Field: verr.Field,
        }
    }
    var bme *balanceMismatchError
    if errors.As(err, &bme) {
        diff := bme.Difference
        return http.StatusConflict, errorBody{
            Error:      "balance_mismatch",
            Message:    bme.Error(),
            Difference: &diff,
        }
    }
    switch {
    case errors.Is(err, service.ErrNotFound):
        return http.StatusNotFound, errorBody{Error: "not_found", Message: err.Error()}
    case errors.Is(err, service.ErrAlreadyExists):
        return http.StatusConflict, errorBody{Error: "already_exists", Message: err.Error()}
    case errors.Is(err, service.ErrReconciled):
        return http.StatusConflict, errorBody{Error: "reconciled", Message: err.Error()}
    case errors.Is(err, service.ErrCircularParent):
        return http.StatusConflict, errorBody{Error: "circular_parent", Message: err.Error()}
    case errors.Is(err, service.ErrNotEditable):
        return http.StatusForbidden, errorBody{Error: "not_editable", Message: err.Error()}
    case errors.Is(err, ledger.ErrLedgerNotFound):
        return http.StatusNotFound, errorBody{Error: "not_found", Message: err.Error()}
    case errors.Is(err, ledger.ErrLedgerExists):
        return http.StatusConflict, errorBody{Error: "already_exists", Message: err.Error()}
    case errors.Is(err, ledger.ErrRemoveActive):
        return http.StatusConflict, errorBody{Error: "cannot_remove_active", Message: err.Error()}
    case errors.Is(err, ledger.ErrNoActiveLedger):
        return http.StatusNotFound, errorBody{Error: "no_active_ledger", Message: err.Error()}
    default:
        return http.StatusInternalServerError, errorBody{Error: "internal", Message: "internal server error"}
    }
}
```

The existing `*service.ValidationError` branch is also moved out of the `switch` for consistency — it was already using `errors.As`.

- [ ] **Step 4: Run the new test and full errors test file**

Run: `go test ./internal/api/ -run TestMapError -v`
Expected: PASS for both `TestMapError`, `TestMapError_BalanceMismatch`, and `TestMapError_InternalHidesDetail`. The existing `TestMapError` table assertions still hold — `body.Difference` is `nil` (zero value of `*int64`) for all existing cases, which JSON-encodes to omitting the field. The existing test never reads `body.Difference`, so it stays green.

- [ ] **Step 5: Verify full build and test suite**

Run: `go build ./... && go test ./...`
Expected: PASS across all packages.

- [ ] **Step 6: Commit**

```bash
git add internal/api/errors.go internal/api/errors_test.go
git commit -m "feat(api): add balanceMismatchError for reconcile gate"
```

---

## Task 2: Add `GET /api/accounts/{id}/unreconciled`

**Files:**
- Create: `internal/api/reconcile.go`
- Create: `internal/api/reconcile_test.go`
- Modify: `internal/api/router.go`

- [ ] **Step 1: Write the failing test**

Create `internal/api/reconcile_test.go`:

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import (
    "encoding/json"
    "fmt"
    "net/http"
    "testing"

    "github.com/hance08/kea/internal/model"
)

func TestHandleListUnreconciled_Empty(t *testing.T) {
    ts, svc, _ := newServerForWrite(t)
    acc := seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 0)

    status, body := getJSON(t, fmt.Sprintf("%s/api/accounts/%d/unreconciled", ts.URL, acc.ID))
    if status != http.StatusOK {
        t.Fatalf("status: got %d, want 200; body=%s", status, body)
    }
    var got struct {
        Entries               []model.ReconcileEntry `json:"entries"`
        LastReconciledBalance int64                  `json:"last_reconciled_balance"`
    }
    if err := json.Unmarshal(body, &got); err != nil {
        t.Fatalf("unmarshal: %v; body=%s", err, body)
    }
    if len(got.Entries) != 0 {
        t.Errorf("entries: got %v, want []", got.Entries)
    }
    if got.LastReconciledBalance != 0 {
        t.Errorf("last_reconciled_balance: got %d, want 0", got.LastReconciledBalance)
    }
}

func TestHandleListUnreconciled_WithTransactions(t *testing.T) {
    ts, svc, _ := newServerForWrite(t)
    src := seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 0)
    seedAccount(t, svc, "Expenses:Coffee", model.AccountTypeExpense, 0)
    seedAccount(t, svc, "Income:Salary", model.AccountTypeRevenue, 0)

    seedTransaction(t, svc, "Assets:Bank", "Expenses:Coffee", 450, 1735689600, "Coffee", model.TxTypeExpense, model.StatusCleared)
    seedTransaction(t, svc, "Income:Salary", "Assets:Bank", 500000, 1735776000, "Salary", model.TxTypeIncome, model.StatusCleared)

    status, body := getJSON(t, fmt.Sprintf("%s/api/accounts/%d/unreconciled", ts.URL, src.ID))
    if status != http.StatusOK {
        t.Fatalf("status: got %d, want 200; body=%s", status, body)
    }
    var got struct {
        Entries               []model.ReconcileEntry `json:"entries"`
        LastReconciledBalance int64                  `json:"last_reconciled_balance"`
    }
    if err := json.Unmarshal(body, &got); err != nil {
        t.Fatalf("unmarshal: %v; body=%s", err, body)
    }
    if len(got.Entries) != 2 {
        t.Fatalf("entries length: got %d, want 2", len(got.Entries))
    }
    if got.LastReconciledBalance != 0 {
        t.Errorf("last_reconciled_balance: got %d, want 0", got.LastReconciledBalance)
    }
}

func TestHandleListUnreconciled_ExcludesReconciled(t *testing.T) {
    ts, svc, st := newServerForWrite(t)
    src := seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 0)
    seedAccount(t, svc, "Expenses:Coffee", model.AccountTypeExpense, 0)

    pinned := seedTransaction(t, svc, "Assets:Bank", "Expenses:Coffee", 450, 1735689600, "Pinned", model.TxTypeExpense, model.StatusCleared)
    open := seedTransaction(t, svc, "Assets:Bank", "Expenses:Coffee", 200, 1735776000, "Open", model.TxTypeExpense, model.StatusCleared)

    seedReconciledTransaction(t, st, src.ID, pinned.ID)
    if err := st.SetLastReconciledBalance(t.Context(), src.ID, 250000); err != nil {
        t.Fatalf("SetLastReconciledBalance: %v", err)
    }

    status, body := getJSON(t, fmt.Sprintf("%s/api/accounts/%d/unreconciled", ts.URL, src.ID))
    if status != http.StatusOK {
        t.Fatalf("status: got %d, want 200; body=%s", status, body)
    }
    var got struct {
        Entries               []model.ReconcileEntry `json:"entries"`
        LastReconciledBalance int64                  `json:"last_reconciled_balance"`
    }
    if err := json.Unmarshal(body, &got); err != nil {
        t.Fatalf("unmarshal: %v; body=%s", err, body)
    }
    if len(got.Entries) != 1 {
        t.Fatalf("entries length: got %d, want 1 (pinned excluded)", len(got.Entries))
    }
    if got.Entries[0].ID != open.ID {
        t.Errorf("entry id: got %d, want %d (open)", got.Entries[0].ID, open.ID)
    }
    if got.LastReconciledBalance != 250000 {
        t.Errorf("last_reconciled_balance: got %d, want 250000", got.LastReconciledBalance)
    }
}

func TestHandleListUnreconciled_IncludesPending(t *testing.T) {
    ts, svc, _ := newServerForWrite(t)
    src := seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 0)
    seedAccount(t, svc, "Expenses:Coffee", model.AccountTypeExpense, 0)

    seedTransaction(t, svc, "Assets:Bank", "Expenses:Coffee", 450, 1735689600, "Pending", model.TxTypeExpense, model.StatusPending)

    status, body := getJSON(t, fmt.Sprintf("%s/api/accounts/%d/unreconciled", ts.URL, src.ID))
    if status != http.StatusOK {
        t.Fatalf("status: got %d, want 200; body=%s", status, body)
    }
    var got struct {
        Entries []model.ReconcileEntry `json:"entries"`
    }
    if err := json.Unmarshal(body, &got); err != nil {
        t.Fatalf("unmarshal: %v; body=%s", err, body)
    }
    if len(got.Entries) != 1 {
        t.Fatalf("entries length: got %d, want 1", len(got.Entries))
    }
    if got.Entries[0].Status != model.StatusPending {
        t.Errorf("entry status: got %v, want Pending", got.Entries[0].Status)
    }
}

func TestHandleListUnreconciled_UnknownAccount(t *testing.T) {
    ts, _, _ := newServerForWrite(t)

    status, body := getJSON(t, fmt.Sprintf("%s/api/accounts/99999/unreconciled", ts.URL))
    if status != http.StatusNotFound {
        t.Fatalf("status: got %d, want 404; body=%s", status, body)
    }
    var got errorBody
    if err := json.Unmarshal(body, &got); err != nil {
        t.Fatalf("unmarshal: %v; body=%s", err, body)
    }
    if got.Error != "not_found" {
        t.Errorf("error code: got %q, want %q", got.Error, "not_found")
    }
}

func TestHandleListUnreconciled_BadPathParam(t *testing.T) {
    ts, _, _ := newServerForWrite(t)

    status, body := getJSON(t, ts.URL+"/api/accounts/not-a-number/unreconciled")
    if status != http.StatusBadRequest {
        t.Fatalf("status: got %d, want 400; body=%s", status, body)
    }
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/api/ -run TestHandleListUnreconciled -v`
Expected: 404 from chi for all (route not registered yet). Tests fail with status assertion errors.

- [ ] **Step 3: Create handler file**

Create `internal/api/reconcile.go`:

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import (
    "net/http"

    "github.com/hance08/kea/internal/model"
)

type unreconciledResponse struct {
    Entries               []*model.ReconcileEntry `json:"entries"`
    LastReconciledBalance int64                   `json:"last_reconciled_balance"`
}

func (s *Server) handleListUnreconciled(w http.ResponseWriter, r *http.Request) error {
    id, err := parseInt64Path(r, "id")
    if err != nil {
        return err
    }
    ctx := r.Context()
    if _, err := s.svc.Account().GetAccountByID(ctx, id); err != nil {
        return err
    }
    entries, lastBalance, err := s.svc.Transaction().GetUnreconciledByAccount(ctx, id)
    if err != nil {
        return err
    }
    if entries == nil {
        entries = []*model.ReconcileEntry{}
    }
    return writeJSON(w, http.StatusOK, unreconciledResponse{
        Entries:               entries,
        LastReconciledBalance: lastBalance,
    })
}
```

The `entries == nil` guard ensures the JSON output is `"entries": []` (not `"entries": null`) when no rows are returned. Required for the empty-array assertions in the tests.

- [ ] **Step 4: Wire the route**

In `internal/api/router.go`, inside the `r.Route("/api", func(r chi.Router) { ... })` block, immediately after the `/accounts/{id}/balance` line, add:

```go
            r.Method(http.MethodGet, "/accounts/{id}/unreconciled", apiHandler(s.handleListUnreconciled))
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/api/ -run TestHandleListUnreconciled -v`
Expected: PASS on all six cases.

- [ ] **Step 6: Verify full build and test suite**

Run: `go build ./... && go test ./...`
Expected: PASS across all packages.

- [ ] **Step 7: Commit**

```bash
git add internal/api/reconcile.go internal/api/reconcile_test.go internal/api/router.go
git commit -m "feat(api): GET /api/accounts/{id}/unreconciled"
```

---

## Task 3: Add `POST /api/accounts/{id}/reconcile/preview`

**Files:**
- Modify: `internal/api/reconcile.go` (add DTOs + handler)
- Modify: `internal/api/reconcile_test.go` (add test functions)
- Modify: `internal/api/router.go` (register route)

- [ ] **Step 1: Write the failing tests**

Append to `internal/api/reconcile_test.go`:

```go
func TestHandleReconcilePreview_ZeroDiff(t *testing.T) {
    ts, svc, _ := newServerForWrite(t)
    src := seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 0)
    seedAccount(t, svc, "Expenses:Coffee", model.AccountTypeExpense, 0)
    seedAccount(t, svc, "Income:Salary", model.AccountTypeRevenue, 0)

    t1 := seedTransaction(t, svc, "Assets:Bank", "Expenses:Coffee", 450, 1735689600, "Coffee", model.TxTypeExpense, model.StatusCleared)
    t2 := seedTransaction(t, svc, "Income:Salary", "Assets:Bank", 500000, 1735776000, "Salary", model.TxTypeIncome, model.StatusCleared)

    // Net for Assets:Bank: -450 + 500000 = 499550. With lastBalance=0, statement 499550 → diff 0.
    payload := map[string]any{"statement_balance": 499550, "transaction_ids": []int64{t1.ID, t2.ID}}
    status, body := postJSON(t, fmt.Sprintf("%s/api/accounts/%d/reconcile/preview", ts.URL, src.ID), payload)
    if status != http.StatusOK {
        t.Fatalf("status: got %d, want 200; body=%s", status, body)
    }
    var got struct {
        Difference int64 `json:"difference"`
    }
    if err := json.Unmarshal(body, &got); err != nil {
        t.Fatalf("unmarshal: %v; body=%s", err, body)
    }
    if got.Difference != 0 {
        t.Errorf("difference: got %d, want 0", got.Difference)
    }
}

func TestHandleReconcilePreview_UnderShoot(t *testing.T) {
    ts, svc, _ := newServerForWrite(t)
    src := seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 0)
    seedAccount(t, svc, "Income:Salary", model.AccountTypeRevenue, 0)
    tx := seedTransaction(t, svc, "Income:Salary", "Assets:Bank", 100000, 1735776000, "Salary", model.TxTypeIncome, model.StatusCleared)

    payload := map[string]any{"statement_balance": 105000, "transaction_ids": []int64{tx.ID}}
    status, body := postJSON(t, fmt.Sprintf("%s/api/accounts/%d/reconcile/preview", ts.URL, src.ID), payload)
    if status != http.StatusOK {
        t.Fatalf("status: got %d, want 200; body=%s", status, body)
    }
    var got struct {
        Difference int64 `json:"difference"`
    }
    if err := json.Unmarshal(body, &got); err != nil {
        t.Fatalf("unmarshal: %v; body=%s", err, body)
    }
    if got.Difference != 5000 {
        t.Errorf("difference: got %d, want 5000", got.Difference)
    }
}

func TestHandleReconcilePreview_OverShoot(t *testing.T) {
    ts, svc, _ := newServerForWrite(t)
    src := seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 0)
    seedAccount(t, svc, "Income:Salary", model.AccountTypeRevenue, 0)
    tx := seedTransaction(t, svc, "Income:Salary", "Assets:Bank", 100000, 1735776000, "Salary", model.TxTypeIncome, model.StatusCleared)

    payload := map[string]any{"statement_balance": 90000, "transaction_ids": []int64{tx.ID}}
    status, body := postJSON(t, fmt.Sprintf("%s/api/accounts/%d/reconcile/preview", ts.URL, src.ID), payload)
    if status != http.StatusOK {
        t.Fatalf("status: got %d, want 200; body=%s", status, body)
    }
    var got struct {
        Difference int64 `json:"difference"`
    }
    if err := json.Unmarshal(body, &got); err != nil {
        t.Fatalf("unmarshal: %v; body=%s", err, body)
    }
    if got.Difference != -10000 {
        t.Errorf("difference: got %d, want -10000", got.Difference)
    }
}

func TestHandleReconcilePreview_EmptyIDs(t *testing.T) {
    ts, svc, _ := newServerForWrite(t)
    src := seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 0)

    payload := map[string]any{"statement_balance": 0, "transaction_ids": []int64{}}
    status, body := postJSON(t, fmt.Sprintf("%s/api/accounts/%d/reconcile/preview", ts.URL, src.ID), payload)
    if status != http.StatusBadRequest {
        t.Fatalf("status: got %d, want 400; body=%s", status, body)
    }
    var got errorBody
    if err := json.Unmarshal(body, &got); err != nil {
        t.Fatalf("unmarshal: %v; body=%s", err, body)
    }
    if got.Error != "validation_failed" {
        t.Errorf("error code: got %q, want validation_failed", got.Error)
    }
    if got.Field != "transactions" {
        t.Errorf("field: got %q, want transactions", got.Field)
    }
}

func TestHandleReconcilePreview_DuplicateIDs(t *testing.T) {
    ts, svc, _ := newServerForWrite(t)
    src := seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 0)
    seedAccount(t, svc, "Expenses:Coffee", model.AccountTypeExpense, 0)
    tx := seedTransaction(t, svc, "Assets:Bank", "Expenses:Coffee", 450, 1735689600, "Coffee", model.TxTypeExpense, model.StatusCleared)

    payload := map[string]any{"statement_balance": -450, "transaction_ids": []int64{tx.ID, tx.ID}}
    status, body := postJSON(t, fmt.Sprintf("%s/api/accounts/%d/reconcile/preview", ts.URL, src.ID), payload)
    if status != http.StatusBadRequest {
        t.Fatalf("status: got %d, want 400; body=%s", status, body)
    }
    var got errorBody
    if err := json.Unmarshal(body, &got); err != nil {
        t.Fatalf("unmarshal: %v; body=%s", err, body)
    }
    if got.Field != "transactions" {
        t.Errorf("field: got %q, want transactions", got.Field)
    }
}

func TestHandleReconcilePreview_IDNotInUnreconciledSet(t *testing.T) {
    ts, svc, st := newServerForWrite(t)
    src := seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 0)
    seedAccount(t, svc, "Expenses:Coffee", model.AccountTypeExpense, 0)
    tx := seedTransaction(t, svc, "Assets:Bank", "Expenses:Coffee", 450, 1735689600, "Coffee", model.TxTypeExpense, model.StatusCleared)
    seedReconciledTransaction(t, st, src.ID, tx.ID)

    payload := map[string]any{"statement_balance": -450, "transaction_ids": []int64{tx.ID}}
    status, body := postJSON(t, fmt.Sprintf("%s/api/accounts/%d/reconcile/preview", ts.URL, src.ID), payload)
    if status != http.StatusBadRequest {
        t.Fatalf("status: got %d, want 400; body=%s", status, body)
    }
    var got errorBody
    if err := json.Unmarshal(body, &got); err != nil {
        t.Fatalf("unmarshal: %v; body=%s", err, body)
    }
    if got.Field != "transactions" {
        t.Errorf("field: got %q, want transactions", got.Field)
    }
}

func TestHandleReconcilePreview_UnknownAccount(t *testing.T) {
    ts, _, _ := newServerForWrite(t)

    payload := map[string]any{"statement_balance": 0, "transaction_ids": []int64{1}}
    status, body := postJSON(t, fmt.Sprintf("%s/api/accounts/99999/reconcile/preview", ts.URL), payload)
    if status != http.StatusNotFound {
        t.Fatalf("status: got %d, want 404; body=%s", status, body)
    }
    var got errorBody
    if err := json.Unmarshal(body, &got); err != nil {
        t.Fatalf("unmarshal: %v; body=%s", err, body)
    }
    if got.Error != "not_found" {
        t.Errorf("error code: got %q, want not_found", got.Error)
    }
}

func TestHandleReconcilePreview_UnknownJSONField(t *testing.T) {
    ts, svc, _ := newServerForWrite(t)
    src := seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 0)

    payload := map[string]any{
        "statement_balance": 0,
        "transaction_ids":   []int64{1},
        "extra_field":       "nope",
    }
    status, _ := postJSON(t, fmt.Sprintf("%s/api/accounts/%d/reconcile/preview", ts.URL, src.ID), payload)
    if status != http.StatusBadRequest {
        t.Fatalf("status: got %d, want 400", status)
    }
}

func TestHandleReconcilePreview_DoesNotWrite(t *testing.T) {
    ts, svc, _ := newServerForWrite(t)
    src := seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 0)
    seedAccount(t, svc, "Expenses:Coffee", model.AccountTypeExpense, 0)
    tx := seedTransaction(t, svc, "Assets:Bank", "Expenses:Coffee", 450, 1735689600, "Coffee", model.TxTypeExpense, model.StatusCleared)

    payload := map[string]any{"statement_balance": -450, "transaction_ids": []int64{tx.ID}}
    if status, body := postJSON(t, fmt.Sprintf("%s/api/accounts/%d/reconcile/preview", ts.URL, src.ID), payload); status != http.StatusOK {
        t.Fatalf("preview: got %d; body=%s", status, body)
    }

    // Re-fetch unreconciled list — entry must still be present.
    status, body := getJSON(t, fmt.Sprintf("%s/api/accounts/%d/unreconciled", ts.URL, src.ID))
    if status != http.StatusOK {
        t.Fatalf("list: got %d; body=%s", status, body)
    }
    var got struct {
        Entries []model.ReconcileEntry `json:"entries"`
    }
    if err := json.Unmarshal(body, &got); err != nil {
        t.Fatalf("unmarshal: %v; body=%s", err, body)
    }
    if len(got.Entries) != 1 {
        t.Errorf("entries: got %d, want 1 (preview must not write)", len(got.Entries))
    }
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/api/ -run TestHandleReconcilePreview -v`
Expected: all FAIL — route not registered (chi returns 404).

- [ ] **Step 3: Add preview DTOs and handler**

Append to `internal/api/reconcile.go`:

```go
type reconcilePreviewRequest struct {
    StatementBalance int64   `json:"statement_balance"`
    TransactionIDs   []int64 `json:"transaction_ids"`
}

type reconcilePreviewResponse struct {
    Difference int64 `json:"difference"`
}

func (s *Server) handleReconcilePreview(w http.ResponseWriter, r *http.Request) error {
    id, err := parseInt64Path(r, "id")
    if err != nil {
        return err
    }
    var req reconcilePreviewRequest
    if err := decodeJSON(r, &req); err != nil {
        return err
    }
    diff, err := s.svc.Transaction().PreviewReconcile(r.Context(), id, req.StatementBalance, req.TransactionIDs)
    if err != nil {
        return err
    }
    return writeJSON(w, http.StatusOK, reconcilePreviewResponse{Difference: diff})
}
```

- [ ] **Step 4: Wire the route**

In `internal/api/router.go`, immediately after the `/accounts/{id}/unreconciled` line added in Task 2, add:

```go
            r.Method(http.MethodPost, "/accounts/{id}/reconcile/preview", apiHandler(s.handleReconcilePreview))
```

Route order matters here: chi resolves the longer literal path `/reconcile/preview` before `/reconcile` only if both routes coexist; since Task 4 will add `/reconcile`, both lines must be present at that point — register `/reconcile/preview` first.

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/api/ -run TestHandleReconcilePreview -v`
Expected: PASS on all nine cases.

- [ ] **Step 6: Verify full build and test suite**

Run: `go build ./... && go test ./...`
Expected: PASS across all packages.

- [ ] **Step 7: Commit**

```bash
git add internal/api/reconcile.go internal/api/reconcile_test.go internal/api/router.go
git commit -m "feat(api): POST /api/accounts/{id}/reconcile/preview"
```

---

## Task 4: Add `POST /api/accounts/{id}/reconcile`

**Files:**
- Modify: `internal/api/reconcile.go` (add DTOs + commit handler)
- Modify: `internal/api/reconcile_test.go` (add test functions)
- Modify: `internal/api/router.go` (register route)

- [ ] **Step 1: Write the failing tests**

Append to `internal/api/reconcile_test.go`:

```go
func TestHandleReconcileCommit_Clean(t *testing.T) {
    ts, svc, st := newServerForWrite(t)
    src := seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 0)
    seedAccount(t, svc, "Income:Salary", model.AccountTypeRevenue, 0)
    tx := seedTransaction(t, svc, "Income:Salary", "Assets:Bank", 100000, 1735776000, "Salary", model.TxTypeIncome, model.StatusCleared)

    payload := map[string]any{"statement_balance": 100000, "transaction_ids": []int64{tx.ID}}
    status, body := postJSON(t, fmt.Sprintf("%s/api/accounts/%d/reconcile", ts.URL, src.ID), payload)
    if status != http.StatusOK {
        t.Fatalf("status: got %d, want 200; body=%s", status, body)
    }
    var got struct {
        ReconciledCount       int   `json:"reconciled_count"`
        Difference            int64 `json:"difference"`
        LastReconciledBalance int64 `json:"last_reconciled_balance"`
    }
    if err := json.Unmarshal(body, &got); err != nil {
        t.Fatalf("unmarshal: %v; body=%s", err, body)
    }
    if got.ReconciledCount != 1 {
        t.Errorf("reconciled_count: got %d, want 1", got.ReconciledCount)
    }
    if got.Difference != 0 {
        t.Errorf("difference: got %d, want 0", got.Difference)
    }
    if got.LastReconciledBalance != 100000 {
        t.Errorf("last_reconciled_balance: got %d, want 100000", got.LastReconciledBalance)
    }

    // Persistence: unreconciled list must exclude the committed ID.
    _, listBody := getJSON(t, fmt.Sprintf("%s/api/accounts/%d/unreconciled", ts.URL, src.ID))
    var list struct {
        Entries []model.ReconcileEntry `json:"entries"`
    }
    if err := json.Unmarshal(listBody, &list); err != nil {
        t.Fatalf("unmarshal list: %v", err)
    }
    if len(list.Entries) != 0 {
        t.Errorf("entries: got %d, want 0 after commit", len(list.Entries))
    }

    persisted, err := st.GetLastReconciledBalance(t.Context(), src.ID)
    if err != nil {
        t.Fatalf("GetLastReconciledBalance: %v", err)
    }
    if persisted != 100000 {
        t.Errorf("persisted balance: got %d, want 100000", persisted)
    }
}

func TestHandleReconcileCommit_AllowMismatchTrue(t *testing.T) {
    ts, svc, st := newServerForWrite(t)
    src := seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 0)
    seedAccount(t, svc, "Income:Salary", model.AccountTypeRevenue, 0)
    tx := seedTransaction(t, svc, "Income:Salary", "Assets:Bank", 100000, 1735776000, "Salary", model.TxTypeIncome, model.StatusCleared)

    // statement 105000, cleared 100000 → diff 5000.
    payload := map[string]any{
        "statement_balance": 105000,
        "transaction_ids":   []int64{tx.ID},
        "allow_mismatch":    true,
    }
    status, body := postJSON(t, fmt.Sprintf("%s/api/accounts/%d/reconcile", ts.URL, src.ID), payload)
    if status != http.StatusOK {
        t.Fatalf("status: got %d, want 200; body=%s", status, body)
    }
    var got struct {
        ReconciledCount       int   `json:"reconciled_count"`
        Difference            int64 `json:"difference"`
        LastReconciledBalance int64 `json:"last_reconciled_balance"`
    }
    if err := json.Unmarshal(body, &got); err != nil {
        t.Fatalf("unmarshal: %v; body=%s", err, body)
    }
    if got.Difference != 5000 {
        t.Errorf("difference: got %d, want 5000", got.Difference)
    }
    // Re-derivation: last_reconciled_balance == statement_balance - diff == 105000 - 5000 == 100000.
    if got.LastReconciledBalance != 100000 {
        t.Errorf("last_reconciled_balance: got %d, want 100000", got.LastReconciledBalance)
    }

    persisted, err := st.GetLastReconciledBalance(t.Context(), src.ID)
    if err != nil {
        t.Fatalf("GetLastReconciledBalance: %v", err)
    }
    if persisted != 100000 {
        t.Errorf("persisted balance: got %d, want 100000", persisted)
    }
}

func TestHandleReconcileCommit_GateRejectsMismatch(t *testing.T) {
    ts, svc, st := newServerForWrite(t)
    src := seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 0)
    seedAccount(t, svc, "Income:Salary", model.AccountTypeRevenue, 0)
    tx := seedTransaction(t, svc, "Income:Salary", "Assets:Bank", 100000, 1735776000, "Salary", model.TxTypeIncome, model.StatusCleared)

    cases := []struct {
        name    string
        payload map[string]any
    }{
        {"absent_flag", map[string]any{
            "statement_balance": 105000,
            "transaction_ids":   []int64{tx.ID},
        }},
        {"explicit_false", map[string]any{
            "statement_balance": 105000,
            "transaction_ids":   []int64{tx.ID},
            "allow_mismatch":    false,
        }},
    }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            status, body := postJSON(t, fmt.Sprintf("%s/api/accounts/%d/reconcile", ts.URL, src.ID), tc.payload)
            if status != http.StatusConflict {
                t.Fatalf("status: got %d, want 409; body=%s", status, body)
            }
            var got errorBody
            if err := json.Unmarshal(body, &got); err != nil {
                t.Fatalf("unmarshal: %v; body=%s", err, body)
            }
            if got.Error != "balance_mismatch" {
                t.Errorf("error code: got %q, want balance_mismatch", got.Error)
            }
            if got.Difference == nil {
                t.Fatalf("difference: got nil, want pointer to 5000")
            }
            if *got.Difference != 5000 {
                t.Errorf("difference: got %d, want 5000", *got.Difference)
            }

            // Critical: no write happened.
            persisted, err := st.GetLastReconciledBalance(t.Context(), src.ID)
            if err != nil {
                t.Fatalf("GetLastReconciledBalance: %v", err)
            }
            if persisted != 0 {
                t.Errorf("persisted balance after rejected gate: got %d, want 0", persisted)
            }
            _, listBody := getJSON(t, fmt.Sprintf("%s/api/accounts/%d/unreconciled", ts.URL, src.ID))
            var list struct {
                Entries []model.ReconcileEntry `json:"entries"`
            }
            if err := json.Unmarshal(listBody, &list); err != nil {
                t.Fatalf("unmarshal list: %v", err)
            }
            if len(list.Entries) != 1 {
                t.Errorf("entries: got %d, want 1 (no write expected)", len(list.Entries))
            }
        })
    }
}

func TestHandleReconcileCommit_ValidationPassthrough(t *testing.T) {
    ts, svc, st := newServerForWrite(t)
    src := seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 0)
    seedAccount(t, svc, "Expenses:Coffee", model.AccountTypeExpense, 0)
    tx := seedTransaction(t, svc, "Assets:Bank", "Expenses:Coffee", 450, 1735689600, "Coffee", model.TxTypeExpense, model.StatusCleared)
    reconciledTx := seedTransaction(t, svc, "Assets:Bank", "Expenses:Coffee", 200, 1735776000, "Pinned", model.TxTypeExpense, model.StatusCleared)
    seedReconciledTransaction(t, st, src.ID, reconciledTx.ID)

    cases := []struct {
        name       string
        payload    map[string]any
        wantStatus int
        wantField  string
    }{
        {"empty_ids_gate_on", map[string]any{
            "statement_balance": -450,
            "transaction_ids":   []int64{},
        }, 400, "transactions"},
        {"empty_ids_gate_off", map[string]any{
            "statement_balance": -450,
            "transaction_ids":   []int64{},
            "allow_mismatch":    true,
        }, 400, "transactions"},
        {"duplicate_gate_on", map[string]any{
            "statement_balance": -450,
            "transaction_ids":   []int64{tx.ID, tx.ID},
        }, 400, "transactions"},
        {"duplicate_gate_off", map[string]any{
            "statement_balance": -450,
            "transaction_ids":   []int64{tx.ID, tx.ID},
            "allow_mismatch":    true,
        }, 400, "transactions"},
        {"already_reconciled_id_gate_on", map[string]any{
            "statement_balance": -200,
            "transaction_ids":   []int64{reconciledTx.ID},
        }, 400, "transactions"},
        {"already_reconciled_id_gate_off", map[string]any{
            "statement_balance": -200,
            "transaction_ids":   []int64{reconciledTx.ID},
            "allow_mismatch":    true,
        }, 400, "transactions"},
    }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            status, body := postJSON(t, fmt.Sprintf("%s/api/accounts/%d/reconcile", ts.URL, src.ID), tc.payload)
            if status != tc.wantStatus {
                t.Fatalf("status: got %d, want %d; body=%s", status, tc.wantStatus, body)
            }
            var got errorBody
            if err := json.Unmarshal(body, &got); err != nil {
                t.Fatalf("unmarshal: %v; body=%s", err, body)
            }
            if got.Field != tc.wantField {
                t.Errorf("field: got %q, want %q", got.Field, tc.wantField)
            }
        })
    }
}

func TestHandleReconcileCommit_UnknownAccount(t *testing.T) {
    ts, _, _ := newServerForWrite(t)

    cases := []struct {
        name    string
        payload map[string]any
    }{
        {"gate_on", map[string]any{"statement_balance": 0, "transaction_ids": []int64{1}}},
        {"gate_off", map[string]any{"statement_balance": 0, "transaction_ids": []int64{1}, "allow_mismatch": true}},
    }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            status, body := postJSON(t, fmt.Sprintf("%s/api/accounts/99999/reconcile", ts.URL), tc.payload)
            if status != http.StatusNotFound {
                t.Fatalf("status: got %d, want 404; body=%s", status, body)
            }
            var got errorBody
            if err := json.Unmarshal(body, &got); err != nil {
                t.Fatalf("unmarshal: %v; body=%s", err, body)
            }
            if got.Error != "not_found" {
                t.Errorf("error code: got %q, want not_found", got.Error)
            }
        })
    }
}

func TestHandleReconcileCommit_UnknownJSONField(t *testing.T) {
    ts, svc, _ := newServerForWrite(t)
    src := seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 0)

    payload := map[string]any{
        "statement_balance": 0,
        "transaction_ids":   []int64{1},
        "extra_field":       "nope",
    }
    status, _ := postJSON(t, fmt.Sprintf("%s/api/accounts/%d/reconcile", ts.URL, src.ID), payload)
    if status != http.StatusBadRequest {
        t.Fatalf("status: got %d, want 400", status)
    }
}

func TestHandleReconcileCommit_Replay(t *testing.T) {
    ts, svc, _ := newServerForWrite(t)
    src := seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 0)
    seedAccount(t, svc, "Income:Salary", model.AccountTypeRevenue, 0)
    tx := seedTransaction(t, svc, "Income:Salary", "Assets:Bank", 100000, 1735776000, "Salary", model.TxTypeIncome, model.StatusCleared)

    payload := map[string]any{"statement_balance": 100000, "transaction_ids": []int64{tx.ID}}

    // First commit succeeds.
    if status, body := postJSON(t, fmt.Sprintf("%s/api/accounts/%d/reconcile", ts.URL, src.ID), payload); status != http.StatusOK {
        t.Fatalf("first commit: got %d; body=%s", status, body)
    }

    // Replay returns 400 — the ID is no longer in the unreconciled set.
    status, body := postJSON(t, fmt.Sprintf("%s/api/accounts/%d/reconcile", ts.URL, src.ID), payload)
    if status != http.StatusBadRequest {
        t.Fatalf("replay: got %d, want 400; body=%s", status, body)
    }
    var got errorBody
    if err := json.Unmarshal(body, &got); err != nil {
        t.Fatalf("unmarshal: %v; body=%s", err, body)
    }
    if got.Field != "transactions" {
        t.Errorf("field: got %q, want transactions", got.Field)
    }
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/api/ -run TestHandleReconcileCommit -v`
Expected: all FAIL — route not registered.

- [ ] **Step 3: Add commit DTOs and handler**

Append to `internal/api/reconcile.go`:

```go
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

func (s *Server) handleReconcileCommit(w http.ResponseWriter, r *http.Request) error {
    id, err := parseInt64Path(r, "id")
    if err != nil {
        return err
    }
    var req reconcileCommitRequest
    if err := decodeJSON(r, &req); err != nil {
        return err
    }
    ctx := r.Context()

    if !req.AllowMismatch {
        diff, err := s.svc.Transaction().PreviewReconcile(ctx, id, req.StatementBalance, req.TransactionIDs)
        if err != nil {
            return err
        }
        if diff != 0 {
            return &balanceMismatchError{Difference: diff}
        }
    }

    diff, err := s.svc.Transaction().ReconcileTransactions(ctx, id, req.StatementBalance, req.TransactionIDs)
    if err != nil {
        return err
    }

    return writeJSON(w, http.StatusOK, reconcileCommitResponse{
        ReconciledCount:       len(req.TransactionIDs),
        Difference:            diff,
        LastReconciledBalance: req.StatementBalance - diff,
    })
}
```

- [ ] **Step 4: Wire the route**

In `internal/api/router.go`, immediately after the `/accounts/{id}/reconcile/preview` line added in Task 3, add:

```go
            r.Method(http.MethodPost, "/accounts/{id}/reconcile", apiHandler(s.handleReconcileCommit))
```

The longer literal `/reconcile/preview` is registered before `/reconcile`, which is the chi-friendly order for these two routes coexisting under the same `/accounts/{id}` prefix.

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/api/ -run TestHandleReconcileCommit -v`
Expected: PASS on all cases (including each `t.Run` subtest).

- [ ] **Step 6: Verify full build and test suite**

Run: `go build ./... && go test ./...`
Expected: PASS across all packages.

- [ ] **Step 7: Commit**

```bash
git add internal/api/reconcile.go internal/api/reconcile_test.go internal/api/router.go
git commit -m "feat(api): POST /api/accounts/{id}/reconcile"
```

---

## Task 5: Final verification

**Files:** None.

- [ ] **Step 1: Run full test suite**

Run: `go test ./...`
Expected: PASS across all packages, no skips, no flaky output. The reconciliation surface adds ~25 new test cases across `internal/api/reconcile_test.go` and `internal/api/errors_test.go`.

- [ ] **Step 2: Run full build**

Run: `go build ./...`
Expected: clean build, no warnings.

- [ ] **Step 3: Sanity check — verify the new routes are mounted**

Run: `grep -n "reconcile\|unreconciled" internal/api/router.go`
Expected output includes three lines registering `GET /accounts/{id}/unreconciled`, `POST /accounts/{id}/reconcile/preview`, and `POST /accounts/{id}/reconcile`.

- [ ] **Step 4: Sanity check — verify ordering in router**

Within the `/api` route block, confirm `/accounts/{id}/reconcile/preview` appears *before* `/accounts/{id}/reconcile` in the source. Chi's resolution is registration-order dependent for path-pattern conflicts; the longer literal must register first.

- [ ] **Step 5: If everything is green, no commit needed.**

Otherwise, fix the failure and commit the fix with a descriptive conventional message (e.g., `fix(api): ...`).

---

## Self-Review Summary

**Spec coverage check:**

| Spec section | Implementing task |
|--------------|-------------------|
| `balanceMismatchError` + `errorBody.Difference` | Task 1 |
| `GET /api/accounts/{id}/unreconciled` (incl. account-existence handler check) | Task 2 |
| `POST /api/accounts/{id}/reconcile/preview` | Task 3 |
| `POST /api/accounts/{id}/reconcile` with `allow_mismatch` gate | Task 4 |
| `last_reconciled_balance` re-derivation | Task 4 (in handler) + Task 4 test `AllowMismatchTrue` (pins the formula) |
| All validation pass-through cases | Tasks 3 + 4 tests |
| Replay safety | Task 4 test `Replay` |
| `errors_test.go` direct + wrapped balance_mismatch | Task 1 |
| Router additions (three routes) | Tasks 2, 3, 4 |

**Out-of-scope items in the spec correctly produce no tasks:** atomic preview-then-commit, unreconcile/undo, bulk reconcile, reconciliation history, registry mutex race, ErrNotFound mismatch, bulk balances, optimistic concurrency, SPA, auth/TLS.

**Type consistency check:** `balanceMismatchError`, `unreconciledResponse`, `reconcilePreviewRequest`, `reconcilePreviewResponse`, `reconcileCommitRequest`, `reconcileCommitResponse` — names appear consistently across the plan. JSON tags (`statement_balance`, `transaction_ids`, `allow_mismatch`, `difference`, `entries`, `last_reconciled_balance`, `reconciled_count`) appear consistently in DTOs, test payloads, and response assertions. Service method names (`GetUnreconciledByAccount`, `PreviewReconcile`, `ReconcileTransactions`, `GetAccountByID`, `GetLastReconciledBalance`, `SetLastReconciledBalance`) match the actual signatures from `internal/service/reconcile_ops.go` and `internal/store/`.
