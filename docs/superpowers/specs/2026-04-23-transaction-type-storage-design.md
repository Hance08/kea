# Transaction Type Storage Design

**Date:** 2026-04-23
**Status:** Approved

## Problem

`DetermineType` in `internal/service/transaction_classifier.go` infers the transaction type from splits at display time. This breaks down for complex multi-split transactions where the split shape is ambiguous and the user's intent cannot be reliably recovered. Type should be stored explicitly on the transaction record.

## Goals

- Store `type` as a non-nullable column on the `transactions` table.
- Capture type as the first prompt in the interactive `kea add` flow (already done) and persist it.
- Add a `--type` flag to `kea add` for flag mode.
- Validate that splits are structurally consistent with the declared type at create and update time.
- Allow type changes in `kea edit` with immediate re-validation feedback.
- Replace all three `DetermineType` display-time call sites with direct field reads.

## Decisions

| Decision | Choice | Rationale |
|---|---|---|
| Type field nullability | Non-nullable, SQL backfill | Eliminates Go-side nil-checks; backfill migration covers all existing rows |
| Validation strictness | Strict structural enforcement | Prevents data corruption; rules are fully enumerable for the three user-facing types |
| Validation rule source | Service `GetTransactionRule` | Single source of truth; extensible when new types are added |
| Migration structure | Two files: schema then backfill | Each file is independently rollback-safe |
| Edit UX | Immediate re-validation on type change | User sees incompatibility before save, not at save |

## Architecture

### Migrations

**`0005_add_transaction_type.up.sql`**
```sql
ALTER TABLE transactions ADD COLUMN type TEXT NOT NULL DEFAULT '';
```
Uses `DEFAULT ''` as a temporary sentinel required by SQLite for NOT NULL on ALTER TABLE. Overwritten by migration 0006 before any Go code reads it.

**`0006_backfill_transaction_type.up.sql`**
Classifies every existing row via a SQL correlated subquery joining splits to accounts — mirrors the logic of `DetermineType`:

```sql
UPDATE transactions SET type = (
  SELECT
    CASE
      WHEN memo_check.has_opening    THEN 'Opening'
      WHEN has_exp AND has_rev       THEN CASE WHEN rev_sum >= exp_sum THEN 'Income' ELSE 'Expense' END
      WHEN has_exp AND al_cnt >= 1   THEN 'Expense'
      WHEN has_rev AND al_cnt >= 1   THEN 'Income'
      WHEN al_cnt >= 2               THEN 'Transfer'
      ELSE 'Other'
    END
  FROM (
    SELECT
      MAX(CASE WHEN a.type = 'E' THEN 1 ELSE 0 END) AS has_exp,
      MAX(CASE WHEN a.type = 'R' THEN 1 ELSE 0 END) AS has_rev,
      SUM(CASE WHEN a.type IN ('A','L') THEN 1 ELSE 0 END) AS al_cnt,
      SUM(CASE WHEN a.type = 'E' THEN ABS(s.amount) ELSE 0 END) AS exp_sum,
      SUM(CASE WHEN a.type = 'R' THEN ABS(s.amount) ELSE 0 END) AS rev_sum
    FROM splits s JOIN accounts a ON s.account_id = a.id
    WHERE s.transaction_id = transactions.id
  ) agg,
  (
    SELECT MAX(CASE WHEN s.memo = 'Opening Balance' THEN 1 ELSE 0 END) AS has_opening
    FROM splits s WHERE s.transaction_id = transactions.id
  ) memo_check
);
```

Corresponding `.down.sql` files revert each change independently.

### Model (`internal/model/transaction.go`)

`Type TransactionType` added to both `Transaction` (persistence struct) and `TransactionDetail` (rich view struct). No changes to `model/types.go` — `TransactionType` and its constants already exist.

### Store (`internal/store/sqlite_transaction.go`)

- `CreateTransactionWithSplits` — `type` added to INSERT columns and params.
- `scanTransactions` — `type` added to SELECT and `rows.Scan`. Cascades to all four read methods that call it.
- `UpdateTransactionBasic` — gains `txType model.TransactionType` param; `type` added to SET clause.
- `internal/repository/interfaces.go` — `UpdateTransactionBasic` signature updated to match.

### Service (`internal/service/`)

**New: `ValidateSplitsMatchType`** in `transaction_classifier.go`

Validates split account types against the declared transaction type. Rules:

| Type | Required split accounts |
|---|---|
| `Expense` | ≥1 `E` account + ≥1 `A` or `L` account |
| `Income` | ≥1 `R` account + ≥1 `A` or `L` account |
| `Transfer` | only `A` or `L` accounts (≥2 splits total) |
| `Opening`, `Other`, etc. | skip (internal types) |

Account types are resolved from `split.AccountType` if present, otherwise via `accRepo.GetAccountByID` (same pattern as `DetermineType`).

Extensibility: adding a new user-facing type (e.g., Investment Deposit) requires adding a new rule branch here and a new entry in the service rule registry — one place to change.

**`CreateTransaction`** (`transaction_ops.go`)

After split resolution: call `ValidateSplitsMatchType(input.Type, resolvedSplits)` before `ValidateSplitsBalance`. Populate `tx.Type = input.Type` when building the `model.Transaction` to persist.

**`UpdateTransactionComplete`** (`transaction_ops.go`)

New signature:
```go
func (ts *TransactionService) UpdateTransactionComplete(
    txID int64,
    description string,
    timestamp int64,
    status model.TransactionStatus,
    txType model.TransactionType,
    splits []model.SplitDetail,
) error
```

Calls `ValidateSplitsMatchType` and passes `txType` to `repo.UpdateTransactionBasic`.

**`CreateSimpleTransaction`** (`transaction_ops.go`)

Gains `txType model.TransactionType` param. Populates `input.Type` in the `TransactionDetail` it builds before calling `CreateTransaction`.

**`DetermineType`** — retained in `transaction_classifier.go`. No longer called in production paths after display logic migration, but kept for testing and future tooling (imports, data repair).

### `cmd/add`

**`addFlags`** — new `Type string` field.

**`NewAddCmd`** — registers `--type` flag:
```go
cmd.Flags().StringVar(&flags.Type, "type", "", "Transaction type: expense, income, transfer")
```

**`hasFlags` detection** — `"type"` added to the `cmd.Flags().Changed(...)` check per Pattern C.

**`runInteractive`** — no structural change. Already calls `PromptTransactionType()` as Step 1 and derives mode. Addition: `input.Type = mode` before returning.

**`runFromFlags`** — parses `flags.Type` into `model.TransactionType`, validates it is one of `expense/income/transfer`, populates `input.Type`. If `--type` is omitted in flag mode, `DetermineType` is called on the two resolved splits to infer the type — preserving backward compatibility with existing flag usage. Type-consistency against accounts is enforced downstream by `ValidateSplitsMatchType`.

**`TransactionProvider` interface** — `CreateSimpleTransaction` signature updated to include `txType`.

**`addTransactionInput`** — new `Type model.TransactionType` field.

### `cmd/transaction/edit`

**Edit menu** — "Change Type" added as a new action. Hidden for Opening Balance transactions (same guard used by existing actions).

**`actionEditType`** (`edit_actions.go`):
1. Call `prompts.PromptTransactionType()`.
2. Call `ts.txSvc.ValidateSplitsMatchType(newType, detail.Splits)` immediately.
3. On failure: `r.view.ShowWarning(...)` — return without mutating `detail`. User sees incompatibility before saving.
4. On success: `detail.Type = newType` + success message.

**`actionSave`** — passes `detail.Type` to the updated `UpdateTransactionComplete`.

**`EditProvider` interface** — `ValidateSplitsMatchType` added; `UpdateTransactionComplete` signature updated.

### Display Logic

Three `DetermineType` call sites replaced with direct field reads:

| File | Before | After |
|---|---|---|
| `cmd/transaction/list.go:117` | `r.svc.DetermineType(detail.Splits)` | `detail.Type` |
| `cmd/transaction/edit_actions.go:42` | `r.txSvc.DetermineType(detail.Splits)` | `detail.Type` |
| `internal/service/report_service.go:67` | `ts.DetermineType(details)` | `tx.Type` |

`DetermineType` removed from `ListProvider` and `EditProvider` interfaces.

## Implementation Order

1. Migrations (`0005`, `0006`) — schema and backfill
2. Model — `Type` field on `Transaction` and `TransactionDetail`
3. Store — INSERT, SELECT (`scanTransactions`), `UpdateTransactionBasic`
4. Service — `ValidateSplitsMatchType`, update `CreateTransaction`, `CreateSimpleTransaction`, `UpdateTransactionComplete`
5. `cmd/add` — `--type` flag, wire `input.Type` through interactive and flag modes
6. `cmd/transaction/edit` — "Change Type" action, update `actionSave`
7. Display logic — replace `DetermineType` calls with field reads, clean up interfaces
