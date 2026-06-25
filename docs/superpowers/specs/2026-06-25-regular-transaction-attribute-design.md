# Regular Transaction Attribute — Design

**Date:** 2026-06-25
**Status:** Approved (brainstorming)
**Next step:** Implementation plan via `writing-plans` skill

## Problem

Users want to distinguish their habitual, predictable cash flow (rent, salary, groceries, subscriptions) from one-off events (gifts received, vacation spending, a surprise bonus). KEA's existing `TransactionStatus` (Pending/Cleared/Reconciled) and `TransactionType` (Expense/Income/Transfer/…) do not capture this.

This is **not** an automation feature. There is no schedule, frequency, or next-due date. It is purely a label set per transaction, used for filtering reports and the SPA list. (A future "recurring rule" automation feature can layer on top later without colliding with this attribute; reserving the word "Recurring" for that future feature is one reason this attribute is named `Regular`.)

## Goal

Add a `Regular` attribute to transactions of type `Income` and `Expense` so users can answer questions like "how much of my monthly spend is regular?" and "what was my regular net cash flow last quarter?"

## Non-goals (v1)

- Automation, scheduling, next-occurrence generation.
- Bulk edit ("mark all Netflix charges as regular").
- CSV import of the `regular` field.
- Report grouping/aggregation by regular vs irregular — only filtering.

## Decisions and naming

- **Name:** `Regular` (boolean). Chosen over "Recurring" so the latter remains available for a future automation feature.
- **Scope:** applies **only** to transactions whose `Type` is `Income` or `Expense`. All other transaction types (`Transfer`, `Opening`, `Deposit`, `Withdrawal`, `Investment`, `Other`) do not have this attribute at all — not "Regular = false" but genuinely absent (NULL in DB, nil pointer in Go, omitted JSON key).
- **Default for new Income/Expense:** `true`. Rationale: most transactions in practice are regular (groceries, salary, fuel, rent); defaulting to true reduces data-entry friction.

## Data model

### Schema

New migration `0011_add_transaction_regular`.

Up (table-rebuild, mirroring the precedent in migration 0007):

1. `CREATE TABLE transactions_new` with all existing columns plus `regular INTEGER` and the CHECK constraint:
   ```sql
   CHECK (
     (type IN ('Income', 'Expense') AND regular IN (0, 1))
     OR (type NOT IN ('Income', 'Expense') AND regular IS NULL)
   )
   ```
2. Backfill via `INSERT … SELECT`: for each row, populate `regular` as `1` when `type IN ('Income','Expense')`, else `NULL`.
3. `DROP TABLE transactions` and `ALTER TABLE transactions_new RENAME TO transactions`.
4. Recreate all indexes that previously existed on `transactions` (capture them up front).

Down: reverse rebuild that drops `regular` and the CHECK constraint.

The column is nullable, has no default, and the CHECK guarantees nothing inconsistent can be written.

### Go model (`internal/model/transaction.go`)

```go
type Transaction struct {
    ID          int64             `json:"id"`
    Timestamp   int64             `json:"timestamp"`
    Description string            `json:"description"`
    Status      TransactionStatus `json:"status"`
    Type        TransactionType   `json:"type"`
    Regular     *bool             `json:"regular,omitempty"`   // NEW
    ExternalID  *string           `json:"external_id,omitempty"`
}
```

Semantics of `*bool`:
- `nil` — attribute does not apply to this transaction (Type ∉ {Income, Expense}).
- `&true` / `&false` — the value, for Income/Expense.

`omitempty` keeps the API surface clean: a `Transfer` transaction's JSON does not even include `"regular"`.

The same `*bool` field is added to:
- `TransactionDetail`
- `CreateSimpleTransactionInput`
- `CreateTransactionFromSplitsInput`
- `UpdateTransactionInput`
- `TransactionListItem` (kept as `*bool` so list rendering can show blank for non-applicable rows)
- `TransactionFilter` (used as `nil` = no filter)

### Invariant

Enforced both in the database (CHECK constraint above) and in the service layer (validation function):

- `Type ∈ {Income, Expense}` ⟺ `Regular != nil`
- `Type ∉ {Income, Expense}` ⟺ `Regular == nil`

## Repository / service layer

### Store (`internal/store/sqlite_transaction.go`)

- `CreateTransaction`: extend INSERT column list and parameters to include `regular`. `database/sql` binds `*bool` to NULL/0/1 directly.
- `GetTransactionByID`, `ListTransactions`, `FilterTransactions`, `ListTransactionsForReport`, and the methods returning `TransactionDetail`: add `regular` to every SELECT column list and scan into `&tx.Regular`.
- `UpdateTransactionBasic`: extend signature to take `regular *bool` and add it to the UPDATE SET clause.

### Filter (`internal/store/sqlite_transaction.go` filter SQL)

When `filter.Regular != nil`, append `AND t.regular = ?` with `1` or `0`. NULL rows are naturally excluded from both `Regular=true` and `Regular=false` results, which is the desired behavior (only Income/Expense are even candidates).

### Validation (`internal/service/transaction_validation.go`)

New function:

```go
func ValidateRegular(txType TransactionType, regular *bool) error
```

Returns `ErrRegularRequired` when type is Income/Expense and `regular == nil`, and `ErrRegularNotApplicable` when type is something else and `regular != nil`. Both errors are added to `internal/service/errors.go`.

Called from every transaction create/update path: `CreateTransaction`, `CreateSimpleTransaction`, `CreateFromSplits`, `UpdateTransaction`.

### Service operations (`internal/service/transaction_ops.go`)

- Create paths: if input type is Income/Expense and caller passed `Regular == nil`, default to `&true` before validation.
- Update paths: if the user's edit changes `Type` out of Income/Expense, clear `Regular = nil`; if the edit changes `Type` into Income/Expense, default `Regular = &true` unless the caller specified.
- `BuildTransactionListItems` copies `Regular` through into each `TransactionListItem`.

## UI surfaces

### CLI `kea add` (`cmd/add.go`)

- New flag: `--regular` / `--no-regular` (a `*bool` via a small helper, since cobra has no built-in tri-state). Default unset.
- Honored only when the resolved type is `Income` or `Expense`; printed-warning + ignored otherwise.
- Interactive (huh) flow: after the Type prompt resolves to Income or Expense, append a `huh.NewSelect[bool]` prompt:
  - Title: "Regular spending?"
  - Options: "Yes" (default) / "No"

### CLI `kea transaction edit` (`cmd/transaction/edit_actions.go`, `edit_types.go`)

- New menu option `OptToggleRegular = "Toggle Regular"`, with `Condition: d.Type == TxTypeIncome || d.Type == TxTypeExpense`.
- Handler `actionToggleRegular` flips `*d.Regular` and persists.
- The existing `actionEditType` is updated to handle the side effect of changing Type: into Income/Expense → `Regular = &true`; out of Income/Expense → `Regular = nil`. Service-layer validation backs this up.

### CLI `kea transaction list` (`cmd/transaction/list.go`) and `kea report`

- New flag `--regular` / `--no-regular` mirroring the API filter.
- Table view adds a `Reg` column:
  - `&true` → `✓`
  - `&false` → `✗`
  - `nil` → blank

### Web SPA — `spa/src/components/transactions/FilterBar.tsx`

- New tri-state select: **Regular: Any / Regular only / Irregular only**. Wires to `?regular=true|false` query param (omitted when "Any").

### API layer (`internal/api/params.go`)

- `parseTransactionFilter` parses `regular=true|false` into `*bool`.

### Web SPA — `TransactionRow.tsx` / list

- Small badge column: `regular === true` → subtle "R" pill; `regular === false` → "1×" pill (one-off); `regular === undefined` → nothing.

### Web SPA — `TransactionForm.tsx` / `SimpleFields.tsx`

- When form type is Income or Expense, show a checkbox **"Regular"**, default checked. With a short helper line: "Tick if this is a habitual income/expense (e.g. salary, rent)."
- Hidden otherwise.
- On type change, clear/restore the field accordingly (mirrors the TUI edit rule).

## Testing

### Migration

- `internal/store/migration_0011_test.go`, modeled on `migration_0009_test.go` and `0010_test.go`. Seed a v0010 DB with mixed transaction types, run migrate-up, assert Income/Expense rows have `regular=1`, others NULL. Run migrate-down, assert column gone.
- CHECK-constraint cases: inserting Income with `regular=NULL` fails; inserting Transfer with `regular=1` fails; inserting Income with `regular=0` succeeds.

### Service (white-box tests using mocks per CLAUDE.md)

- `transaction_validation_test.go`: table-driven cases for `ValidateRegular` covering all 8 transaction types × {nil, &true, &false}.
- `transaction_ops_test.go`:
  - Create Expense with `Regular == nil` → stored as `&true`.
  - Create Transfer with `Regular = &true` → rejected with `ErrRegularNotApplicable`.
  - Update Expense → Transfer clears `Regular` to nil.
  - Update Transfer → Income defaults `Regular` to `&true`.
- `transaction_filter_test.go`: filter by `Regular=&true` returns only regular Income/Expense rows; filter by `&false` excludes NULL rows.
- `sqlite_transaction_test.go`: round-trip Income with `&false`, Transfer with `nil`, and re-fetch.

### API

- `internal/api/params_test.go`: `parseTransactionFilter` parses `regular=true|false`.
- `transactions_test.go`: list endpoint filters by `regular`; create/update endpoint round-trips the field.

### SPA

- `spa/src/test/transactions.list.test.tsx` (extend) and new `transactions.regular.test.tsx`:
  - Filter renders only matching rows.
  - Form hides the checkbox when type ≠ Income/Expense.
  - Switching type clears the field.

### JSON round-trip (`internal/model/json_test.go`)

- Transfer marshals without `"regular"` key.
- Income marshals with `"regular": true|false`.
- Income unmarshalled with no `regular` key → `nil`; verify the service layer rejects this state on save.

## Rollout and risks

- **Backwards compatibility:** the API field is additive and `omitempty`; older SPA bundles ignore it.
- **CLI compatibility:** new flags are additive; default behavior is unchanged for callers that omit them.
- **Backup safety:** the migration rebuilds the `transactions` table; KEA's existing pre-startup backup (`internal/backup/`) covers this — no new backup code needed.
- **Embedded SPA bundle:** `internal/web/dist/` must be refreshed and committed as a final step, matching the existing workflow (see commit `78a5c22 build(spa): refresh embedded bundle`).
- **Investment re-classification edge case:** migrations 0009/0010 backfill `Investment` type from rows previously labelled Income/Expense. If a similar future backfill flips a transaction's type out of Income/Expense, the service must null its `Regular` to preserve the invariant. The CHECK constraint will refuse any inconsistent state, surfacing such bugs immediately.
- **User comprehension:** "Regular" is close to "Status" semantically for new users. The SPA form helper text and the column label in the list view exist for this reason.
