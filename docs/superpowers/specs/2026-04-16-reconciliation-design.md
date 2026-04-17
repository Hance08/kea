# Reconciliation Feature Design

_Date: 2026-04-16_

## Overview

Reconciliation is the process of comparing your KEA records against an external statement (bank, credit card, etc.) and marking matched transactions as immutable. This document covers the full design of `kea reconcile`, including the interactive TUI, non-interactive agent mode, service layer, and repository layer.

---

## Decisions

| Question | Decision |
|---|---|
| Supported account types | All (A, L, R, E, C) |
| Session resumability | One-shot — no session state persisted |
| Balance mismatch behaviour | Soft warning — user can confirm even with non-zero difference |
| Command placement | Top-level: `kea reconcile <account-name>` |
| Entry inputs | Account name (arg) + statement ending balance (interactive prompt or `--balance` flag) |
| TUI library | `bubbletea` (already an indirect dep via `huh`) |
| TUI layout | Compact header with status badge, checkbox `[✓]`/`[ ]` style |
| Agent mode | Fully non-interactive when `--balance`, `--ids`, and `--json` are all provided |

---

## Command & Entry Point

### Interactive mode (default)

```
$ kea reconcile "Assets:Checking"
  Statement ending balance: 2450.00    ← huh input prompt
  [bubbletea TUI opens]
```

### Non-interactive / agent mode

```bash
kea reconcile "Assets:Checking" \
  --balance 2450.00 \
  --ids 12,15,18,22 \
  --force \
  --json
```

When `--balance` and `--ids` are both present the TUI is skipped entirely. `--force` suppresses the balance-mismatch warning. `--json` emits machine-readable output.

### Flags

| Flag | Type | Description |
|---|---|---|
| `--balance` | `string` | Statement ending balance (parsed via `utils.ParseAmount`) |
| `--ids` | `string` | Comma-separated transaction IDs to reconcile (non-interactive mode) |
| `--force` | `bool` | Skip balance-mismatch confirmation (non-interactive mode) |
| `--json` | `bool` | Emit JSON result instead of styled output |

### JSON output

```json
{
  "account": "Assets:Checking",
  "reconciled_count": 4,
  "difference": 0
}
```

### File layout

Follows the existing `cmd/add.go` + `cmd/add_actions.go` pattern:

```
cmd/reconcile.go          — cobra command, flag binding, runner construction
cmd/reconcile_actions.go  — runner struct, Run(), interactive/non-interactive branching
```

A `ReconcileProvider` interface gates the service dependency on the runner, keeping it testable without a real database.

---

## TUI Screen (bubbletea)

### Layout

```
Assets:Checking                        [OFF BY $450.00]

STATEMENT: $2,450.00 · 6 UNRECONCILED
──────────────────────────────────────
[✓] Apr 01  Rent payment          -$1,200.00
[ ] Apr 03  Grocery Store             -$87.50
[✓] Apr 05  Salary deposit        +$3,200.00
[ ] Apr 08  Electric bill             -$95.00
[ ] Apr 10  Coffee shop                -$4.80   ← cursor
[ ] Apr 12  Internet bill             -$60.00
──────────────────────────────────────
Cleared $2,000.00 · Remaining $450.00

space toggle · enter finish · ↑↓ · q quit
```

- Status badge turns green and reads `BALANCED` when difference = 0
- `Remaining` label turns green when difference = 0, red when non-zero
- Cursor row is highlighted

### Keybindings

| Key | Action |
|---|---|
| `↑` / `k` | Move cursor up |
| `↓` / `j` | Move cursor down |
| `space` | Toggle selected transaction |
| `enter` | Finish — confirm and reconcile |
| `q` / `esc` | Quit without reconciling |

### Finish flow

1. User presses `enter`
2. If difference ≠ 0: show inline warning `"You're off by $X. Confirm anyway? (y/n)"` and wait for keypress
3. If `y` (or difference = 0): call service, show success message, exit TUI
4. If `n`: return to the checklist

### File layout

```
ui/reconcile/model.go     — bubbletea Model, Init, Update, View
ui/reconcile/keys.go      — key bindings
```

---

## Service Layer

### Method signature

```go
// ReconcileTransactions marks the given transactions as reconciled for an account.
// Returns the difference between statementBalance and the sum of the selected
// splits for the account. A non-zero difference is informational — the caller
// decides whether to warn or abort.
func (ts *TransactionService) ReconcileTransactions(
    accountID int64,
    statementBalance int64,
    txIDs []int64,
) (difference int64, err error)
```

### Logic

1. Verify the account exists via `GetAccountByID`.
2. Fetch unreconciled transactions via `GetUnreconciledTransactionsByAccount(accountID)`.
3. Build a set of valid IDs from step 2. Reject any `txID` not in the set (unknown or already reconciled) — return an error before any writes.
4. Compute `clearedBalance` = sum of the selected split amounts for `accountID` across the chosen transactions.
5. Call `BulkUpdateTransactionStatus(txIDs, StatusReconciled)` atomically.
6. Return `difference = statementBalance - clearedBalance`.

The method never gates on `difference` — it always reconciles if the IDs are valid. Soft-warning logic lives in the command layer.

---

## Repository Layer

Two new methods added to `TransactionRepository` in `internal/repository/interfaces.go`:

```go
// GetUnreconciledTransactionsByAccount returns all Pending and Cleared
// transactions that have a split touching the given account.
// StatusReconciled transactions are excluded.
GetUnreconciledTransactionsByAccount(accountID int64) ([]*model.Transaction, error)

// BulkUpdateTransactionStatus sets the status of all given transaction IDs
// in a single atomic UPDATE statement.
BulkUpdateTransactionStatus(txIDs []int64, status model.TransactionStatus) error
```

### SQL notes

`GetUnreconciledTransactionsByAccount` joins `transactions` with `splits` on `account_id` and filters `status IN (0, 1)`.

`BulkUpdateTransactionStatus` issues `UPDATE transactions SET status = ? WHERE id IN (...)` inside a database transaction to guarantee atomicity.

Both methods are implemented in `internal/store/`.

---

## Testing

Service-layer tests follow the existing white-box pattern (`package service`) with mocks from `testhelper_test.go`.

### New mock methods

`mockTransactionRepo` gains:
- `GetUnreconciledTransactionsByAccount(accountID int64)` — injectable error map + return slice
- `BulkUpdateTransactionStatus(txIDs []int64, status model.TransactionStatus)` — call recorder + injectable error

### Test cases for `ReconcileTransactions`

| Case | Expected |
|---|---|
| Valid IDs, difference = 0 | returns 0, no error, bulk update called |
| Valid IDs, difference ≠ 0 | returns difference, no error, bulk update called |
| Unknown txID in request | error returned, bulk update not called |
| Already-reconciled txID | error returned, bulk update not called |
| Empty txIDs slice | error returned |
| Account not found | error returned |

No store-level integration tests are added in this feature (that gap exists across the codebase and is out of scope here).

---

## Data Flow Summary

```
kea reconcile "Assets:Checking"
  │
  ├─ huh prompt → statementBalance
  │
  ├─ svc.Transaction().GetUnreconciledTransactionsByAccount(accountID)
  │     → []Transaction   (populates TUI list)
  │
  ├─ [user toggles transactions in bubbletea TUI]
  │
  ├─ user presses Enter
  │     → if difference ≠ 0: soft warning y/n
  │
  └─ svc.Transaction().ReconcileTransactions(accountID, statementBalance, selectedIDs)
        ├─ validate IDs
        ├─ BulkUpdateTransactionStatus(selectedIDs, StatusReconciled)
        └─ return difference
```
