# Regular Transaction Attribute Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a nullable `Regular` boolean attribute to transactions so users can flag Income and Expense transactions as habitual ("regular") vs one-off, with filter and display support across CLI, web API, and SPA.

**Architecture:** Store as nullable `INTEGER` column on `transactions` with a DB-level CHECK constraint enforcing the invariant (`Regular` is set ⟺ `Type ∈ {Income, Expense}`). Carry as `*bool` through Go models, defaults to `true` for new Income/Expense in the service layer, and is cleared to `nil` when a transaction's type changes out of Income/Expense. SPA mirrors the same rules in its form and adds a tri-state filter.

**Tech Stack:** Go 1.x, SQLite (`mattn/go-sqlite3`), golang-migrate, charmbracelet/huh, React + TypeScript (SPA), Vite, Vitest.

**Spec:** [docs/superpowers/specs/2026-06-25-regular-transaction-attribute-design.md](../specs/2026-06-25-regular-transaction-attribute-design.md)

---

## File Map

**Create:**
- `migrations/0011_add_transaction_regular.up.sql`
- `migrations/0011_add_transaction_regular.down.sql`
- `internal/store/migration_0011_test.go`
- `spa/src/test/transactions.regular.test.tsx`

**Modify (Go — model):**
- `internal/model/transaction.go` — add `Regular *bool` to `Transaction`, `TransactionDetail`, `TransactionListItem`
- `internal/model/transaction_list_item.go` — change `Regular` to `*bool`
- `internal/model/input.go` — add `Regular *bool` to `CreateSimpleTransactionInput`, `CreateTransactionFromSplitsInput`, `UpdateTransactionInput`
- `internal/model/transaction_filter.go` — add `Regular *bool`

**Modify (Go — service):**
- `internal/service/errors.go` — `ErrRegularRequired`, `ErrRegularNotApplicable`
- `internal/service/transaction_validation.go` — `ValidateRegular`
- `internal/service/transaction_validation_test.go` — table-driven cases
- `internal/service/transaction_ops.go` — call `ValidateRegular`, apply default, clear-on-type-change
- `internal/service/transaction_classifier.go` — `BuildTransactionListItems` copies `Regular`
- `internal/service/testhelper_test.go` — mock additions if needed (none expected)

**Modify (Go — repository / store):**
- `internal/repository/interfaces.go` — `UpdateTransactionBasic` signature gains `regular *bool`
- `internal/store/sqlite_transaction.go` — every CREATE/SELECT/UPDATE/filter query that touches `transactions` columns; `scanTransactions` helper
- `internal/store/sqlite_transaction_test.go` — round-trip tests
- `internal/store/sqlite_transaction_filter_test.go` — filter tests

**Modify (Go — API):**
- `internal/api/params.go` — `parseTransactionFilter` parses `regular=true|false`
- `internal/api/params_test.go` — parse tests
- `internal/api/transactions_test.go` — list filter test
- `internal/api/transactions_write_test.go` — round-trip test

**Modify (Go — CLI):**
- `cmd/add.go` — `--regular` flag wiring
- `cmd/add_types.go` — flag struct field
- `cmd/add_actions.go` — flag → input plumbing + interactive prompt
- `cmd/transaction/edit_types.go` — `OptToggleRegular` constant
- `cmd/transaction/edit.go` — menu entry + `Condition`
- `cmd/transaction/edit_actions.go` — `actionToggleRegular`, `actionEditType` side effect
- `cmd/transaction/list.go` — `--regular`/`--no-regular` flag, `Reg` column
- `cmd/report.go` — `--regular`/`--no-regular` flag passed into report filter

**Modify (SPA):**
- `spa/src/lib/types.ts` — `Transaction` / `TransactionFilter` / form types
- `spa/src/components/transactions/FilterBar.tsx` — tri-state select
- `spa/src/components/transactions/TransactionRow.tsx` — badge
- `spa/src/components/transactions/SimpleFields.tsx` — checkbox
- `spa/src/components/transactions/TransactionForm.tsx` — wiring + type-change side effect
- `spa/src/test/transactions.list.test.tsx` — column display assertions

**Modify (embedded bundle):**
- `internal/web/dist/` — rebuilt SPA assets (final step)

---

## Task 1: Add `Regular *bool` to Go model structs

**Files:**
- Modify: `internal/model/transaction.go`
- Modify: `internal/model/transaction_list_item.go`
- Modify: `internal/model/input.go`
- Modify: `internal/model/transaction_filter.go`

- [ ] **Step 1.1: Edit `internal/model/transaction.go`**

Add `Regular *bool` to `Transaction` and `TransactionDetail`. Final shape:

```go
type Transaction struct {
    ID          int64             `json:"id"`
    Timestamp   int64             `json:"timestamp"`
    Description string            `json:"description"`
    Status      TransactionStatus `json:"status"`
    Type        TransactionType   `json:"type"`
    Regular     *bool             `json:"regular,omitempty"`
    ExternalID  *string           `json:"external_id,omitempty"`
}

type TransactionDetail struct {
    ID          int64             `json:"id"`
    Timestamp   int64             `json:"timestamp"`
    Description string            `json:"description"`
    Status      TransactionStatus `json:"status"`
    Type        TransactionType   `json:"type"`
    Regular     *bool             `json:"regular,omitempty"`
    Splits      []SplitDetail     `json:"splits"`
}
```

- [ ] **Step 1.2: Edit `internal/model/transaction_list_item.go`**

```go
type TransactionListItem struct {
    ID            int64
    Date          string
    Type          string
    Account       string
    OffsetAccount string
    Description   string
    Amount        int64
    Currency      string
    Status        string
    Regular       *bool
}
```

- [ ] **Step 1.3: Edit `internal/model/input.go`**

Add `Regular *bool` to all four input structs (use `json:"regular,omitempty"` for those with JSON tags):

```go
type CreateSimpleTransactionInput struct {
    FromAccount string
    ToAccount   string
    Amount      int64
    Description string
    Timestamp   int64
    Status      TransactionStatus
    Type        TransactionType
    Regular     *bool
}

type CreateTransactionFromSplitsInput struct {
    Splits      []SplitDetail     `json:"splits"`
    Description string            `json:"description"`
    Timestamp   int64             `json:"timestamp"`
    Status      TransactionStatus `json:"status"`
    Type        TransactionType   `json:"type"`
    Regular     *bool             `json:"regular,omitempty"`
}

type UpdateTransactionInput struct {
    ID          int64             `json:"-"`
    Description string            `json:"description"`
    Timestamp   int64             `json:"timestamp"`
    Status      TransactionStatus `json:"status"`
    Type        TransactionType   `json:"type"`
    Regular     *bool             `json:"regular,omitempty"`
    Splits      []SplitDetail     `json:"splits"`
}
```

- [ ] **Step 1.4: Edit `internal/model/transaction_filter.go`**

```go
type TransactionFilter struct {
    AccountID   *int64
    Type        *TransactionType
    Status      *TransactionStatus
    StartTime   *int64
    EndTime     *int64
    Description *string
    Regular     *bool
}
```

- [ ] **Step 1.5: Verify the package still compiles**

Run: `go build ./internal/model/...`
Expected: no errors.

- [ ] **Step 1.6: Verify the project still compiles**

Run: `go build ./...`
Expected: a small number of compile errors in `internal/store/sqlite_transaction.go` (Scan/Insert mismatches with the new field), and in `cmd/transaction/list.go` if the view code reads `TransactionListItem.Regular`. Note them — they are addressed in later tasks. The build does NOT need to be green at the end of this task.

- [ ] **Step 1.7: Commit**

```bash
git add internal/model/transaction.go internal/model/transaction_list_item.go internal/model/input.go internal/model/transaction_filter.go
git commit -m "model: add Regular *bool to transaction structs"
```

---

## Task 2: Add error sentinels for the Regular invariant

**Files:**
- Modify: `internal/service/errors.go`
- Test: `internal/service/errors_test.go`

- [ ] **Step 2.1: Add error sentinels**

Edit `internal/service/errors.go`. Inside the existing `var (...)` block, add:

```go
ErrRegularRequired     = errors.New("regular attribute is required for Income/Expense transactions")
ErrRegularNotApplicable = errors.New("regular attribute is not applicable to this transaction type")
```

- [ ] **Step 2.2: Verify build**

Run: `go build ./internal/service/...`
Expected: no errors.

- [ ] **Step 2.3: Commit**

```bash
git add internal/service/errors.go
git commit -m "service: add ErrRegularRequired and ErrRegularNotApplicable"
```

---

## Task 3: Implement `ValidateRegular`

**Files:**
- Modify: `internal/service/transaction_validation.go`
- Test: `internal/service/transaction_validation_test.go`

- [ ] **Step 3.1: Write the failing test**

Append to `internal/service/transaction_validation_test.go`:

```go
func TestValidateRegular(t *testing.T) {
    t.Parallel()
    boolPtr := func(b bool) *bool { return &b }

    tests := []struct {
        name    string
        txType  model.TransactionType
        regular *bool
        wantErr error
    }{
        // Income/Expense: Regular MUST be set.
        {"income with true", model.TxTypeIncome, boolPtr(true), nil},
        {"income with false", model.TxTypeIncome, boolPtr(false), nil},
        {"income with nil", model.TxTypeIncome, nil, service.ErrRegularRequired},
        {"expense with true", model.TxTypeExpense, boolPtr(true), nil},
        {"expense with false", model.TxTypeExpense, boolPtr(false), nil},
        {"expense with nil", model.TxTypeExpense, nil, service.ErrRegularRequired},

        // All other types: Regular MUST be nil.
        {"transfer with nil", model.TxTypeTransfer, nil, nil},
        {"transfer with true", model.TxTypeTransfer, boolPtr(true), service.ErrRegularNotApplicable},
        {"transfer with false", model.TxTypeTransfer, boolPtr(false), service.ErrRegularNotApplicable},
        {"opening with nil", model.TxTypeOpening, nil, nil},
        {"opening with true", model.TxTypeOpening, boolPtr(true), service.ErrRegularNotApplicable},
        {"deposit with nil", model.TxTypeDeposit, nil, nil},
        {"deposit with true", model.TxTypeDeposit, boolPtr(true), service.ErrRegularNotApplicable},
        {"withdrawal with nil", model.TxTypeWithdrawal, nil, nil},
        {"withdrawal with false", model.TxTypeWithdrawal, boolPtr(false), service.ErrRegularNotApplicable},
        {"investment with nil", model.TxTypeInvestment, nil, nil},
        {"investment with true", model.TxTypeInvestment, boolPtr(true), service.ErrRegularNotApplicable},
        {"other with nil", model.TxTypeOther, nil, nil},
        {"other with true", model.TxTypeOther, boolPtr(true), service.ErrRegularNotApplicable},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := service.ValidateRegular(tt.txType, tt.regular)
            if tt.wantErr == nil {
                if err != nil {
                    t.Fatalf("ValidateRegular(%s, %v) = %v, want nil", tt.txType, tt.regular, err)
                }
                return
            }
            if !errors.Is(err, tt.wantErr) {
                t.Fatalf("ValidateRegular(%s, %v) = %v, want %v", tt.txType, tt.regular, err, tt.wantErr)
            }
        })
    }
}
```

Note: this file is white-box (`package service`) per CLAUDE.md, so reference the symbols directly (`ValidateRegular`, `ErrRegularRequired`, etc.) without the `service.` qualifier — adjust accordingly:

```go
err := ValidateRegular(tt.txType, tt.regular)
// and: ErrRegularRequired / ErrRegularNotApplicable
```

- [ ] **Step 3.2: Run test to verify it fails**

Run: `go test ./internal/service/ -run TestValidateRegular -v`
Expected: build error — `undefined: ValidateRegular`.

- [ ] **Step 3.3: Implement `ValidateRegular`**

Append to `internal/service/transaction_validation.go`:

```go
// ValidateRegular enforces the invariant that the Regular attribute is set
// (non-nil) for Income and Expense transactions, and nil for every other
// transaction type.
func ValidateRegular(txType model.TransactionType, regular *bool) error {
    isIncomeOrExpense := txType == model.TxTypeIncome || txType == model.TxTypeExpense
    if isIncomeOrExpense && regular == nil {
        return ErrRegularRequired
    }
    if !isIncomeOrExpense && regular != nil {
        return ErrRegularNotApplicable
    }
    return nil
}
```

- [ ] **Step 3.4: Run test to verify it passes**

Run: `go test ./internal/service/ -run TestValidateRegular -v`
Expected: PASS (all 18 sub-tests).

- [ ] **Step 3.5: Commit**

```bash
git add internal/service/transaction_validation.go internal/service/transaction_validation_test.go
git commit -m "service: add ValidateRegular invariant check"
```

---

## Task 4: Wire `Regular` defaulting and clearing into create/update service ops

**Files:**
- Modify: `internal/service/transaction_ops.go`
- Test: `internal/service/transaction_ops_test.go`

- [ ] **Step 4.1: Write the failing tests**

Append to `internal/service/transaction_ops_test.go`:

```go
func TestCreateTransaction_RegularDefaultsForExpense(t *testing.T) {
    ts, _, txRepo := newTestTransactionService(t)
    ctx := context.Background()

    // Caller passes Regular = nil for an Expense; service should default to &true.
    detail := model.TransactionDetail{
        Description: "Coffee",
        Status:      model.StatusCleared,
        Type:        model.TxTypeExpense,
        Timestamp:   1_700_000_000,
        Splits: []model.SplitDetail{
            {AccountName: "Expenses:Coffee", Amount: 150},
            {AccountName: "Assets:Cash", Amount: -150},
        },
    }
    if _, err := ts.CreateTransaction(ctx, detail); err != nil {
        t.Fatalf("CreateTransaction returned error: %v", err)
    }
    if len(txRepo.createdTxs) != 1 {
        t.Fatalf("expected one tx, got %d", len(txRepo.createdTxs))
    }
    got := txRepo.createdTxs[0]
    if got.Regular == nil || *got.Regular != true {
        t.Fatalf("expected Regular = &true, got %v", got.Regular)
    }
}

func TestCreateTransaction_RegularRejectedForTransfer(t *testing.T) {
    ts, _, _ := newTestTransactionService(t)
    ctx := context.Background()

    boolPtr := func(b bool) *bool { return &b }
    detail := model.TransactionDetail{
        Description: "ATM",
        Status:      model.StatusCleared,
        Type:        model.TxTypeTransfer,
        Timestamp:   1_700_000_000,
        Regular:     boolPtr(true),
        Splits: []model.SplitDetail{
            {AccountName: "Assets:Cash", Amount: 1_000},
            {AccountName: "Assets:Bank", Amount: -1_000},
        },
    }
    _, err := ts.CreateTransaction(ctx, detail)
    if !errors.Is(err, ErrRegularNotApplicable) {
        t.Fatalf("expected ErrRegularNotApplicable, got %v", err)
    }
}

func TestUpdateTransactionComplete_ClearsRegularWhenTypeMovesOut(t *testing.T) {
    ts, _, txRepo := newTestTransactionService(t)
    ctx := context.Background()

    // Seed an existing Income transaction with Regular = true.
    boolPtr := func(b bool) *bool { return &b }
    existingID := txRepo.seedTx(model.Transaction{
        ID: 42, Description: "Salary",
        Status: model.StatusCleared, Type: model.TxTypeIncome,
        Regular: boolPtr(true), Timestamp: 1_700_000_000,
    })

    // Caller updates Type to Transfer and forgets to nil Regular — service
    // must clear it instead of rejecting.
    err := ts.UpdateTransactionComplete(ctx, model.UpdateTransactionInput{
        ID:          existingID,
        Description: "Internal move",
        Timestamp:   1_700_000_000,
        Status:      model.StatusCleared,
        Type:        model.TxTypeTransfer,
        Regular:     boolPtr(true),
        Splits: []model.SplitDetail{
            {AccountName: "Assets:Cash", Amount: 1_000},
            {AccountName: "Assets:Bank", Amount: -1_000},
        },
    })
    if err != nil {
        t.Fatalf("UpdateTransactionComplete returned error: %v", err)
    }
    if got := txRepo.updateBasicCalls[len(txRepo.updateBasicCalls)-1].Regular; got != nil {
        t.Fatalf("expected Regular to be cleared to nil, got %v", got)
    }
}

func TestUpdateTransactionComplete_DefaultsRegularWhenTypeMovesIn(t *testing.T) {
    ts, _, txRepo := newTestTransactionService(t)
    ctx := context.Background()

    // Seed an existing Transfer (Regular is nil).
    existingID := txRepo.seedTx(model.Transaction{
        ID: 43, Description: "Internal",
        Status: model.StatusCleared, Type: model.TxTypeTransfer,
        Timestamp: 1_700_000_000,
    })

    err := ts.UpdateTransactionComplete(ctx, model.UpdateTransactionInput{
        ID:          existingID,
        Description: "Salary",
        Timestamp:   1_700_000_000,
        Status:      model.StatusCleared,
        Type:        model.TxTypeIncome,
        // Regular is intentionally nil — service should default to &true.
        Splits: []model.SplitDetail{
            {AccountName: "Assets:Bank", Amount: 100_000},
            {AccountName: "Revenue:Salary", Amount: -100_000},
        },
    })
    if err != nil {
        t.Fatalf("UpdateTransactionComplete returned error: %v", err)
    }
    last := txRepo.updateBasicCalls[len(txRepo.updateBasicCalls)-1]
    if last.Regular == nil || *last.Regular != true {
        t.Fatalf("expected Regular default to &true, got %v", last.Regular)
    }
}
```

(If `mockTransactionRepo` does not yet expose `createdTxs`, `updateBasicCalls`, or `seedTx`, extend `internal/service/testhelper_test.go` to track them — the existing helpers already record calls; mirror the pattern. Examples:
- Add field `createdTxs []model.Transaction` and append in `CreateTransactionWithSplits`.
- Add struct `updateBasicCall { ID int64; Description string; Timestamp int64; Status model.TransactionStatus; Type model.TransactionType; Regular *bool }` and slice `updateBasicCalls []updateBasicCall`; append in `UpdateTransactionBasic`.
- Add helper `seedTx(tx model.Transaction) int64` that stores the tx in the map and returns the ID.)

- [ ] **Step 4.2: Run tests to verify they fail**

Run: `go test ./internal/service/ -run "TestCreateTransaction_Regular|TestUpdateTransactionComplete_.*Regular" -v`
Expected: failures because (a) defaulting/clearing logic doesn't exist yet, and (b) `UpdateTransactionBasic` doesn't yet accept `regular`.

- [ ] **Step 4.3: Update the repository interface**

Edit `internal/repository/interfaces.go`. Find `UpdateTransactionBasic` and add `regular *bool` as the last parameter:

```go
UpdateTransactionBasic(ctx context.Context, txID int64, description string, timestamp int64, status model.TransactionStatus, txType model.TransactionType, regular *bool) error
```

- [ ] **Step 4.4: Update the SQLite store stub (full impl in Task 7)**

Edit `internal/store/sqlite_transaction.go` `UpdateTransactionBasic` signature only — add the `regular *bool` parameter; leave the SQL unchanged for now. We'll wire the SQL in Task 7.

```go
func (s *Store) UpdateTransactionBasic(ctx context.Context, txID int64, description string, timestamp int64, status model.TransactionStatus, txType model.TransactionType, regular *bool) error {
    // existing body unchanged; regular is ignored until Task 7.
    _ = regular
    // ...
}
```

- [ ] **Step 4.5: Update the mock and any other implementers**

Search for other `UpdateTransactionBasic` implementations:

Run: `grep -rn "UpdateTransactionBasic" --include="*.go"`

Expected hits: the interface, the SQLite store (just updated), the mock in `internal/service/testhelper_test.go`, and the call site in `internal/service/transaction_ops.go`. Update the mock to accept and record the new parameter.

- [ ] **Step 4.6: Wire defaulting + clearing in `transaction_ops.go`**

In `CreateTransaction` (around line 100 — just after `ValidateSplitsMatchType` succeeds), default `Regular` then validate:

```go
// Default Regular = &true for new Income/Expense when caller did not specify.
if (input.Type == model.TxTypeIncome || input.Type == model.TxTypeExpense) && input.Regular == nil {
    t := true
    input.Regular = &t
}
if err := ValidateRegular(input.Type, input.Regular); err != nil {
    return 0, err
}
```

Then include `Regular: input.Regular` in the `model.Transaction` literal further down:

```go
tx := model.Transaction{
    Timestamp:   input.Timestamp,
    Description: input.Description,
    Status:      input.Status,
    Type:        input.Type,
    Regular:     input.Regular,
}
```

In `CreateSimpleTransaction`, after `txType` is resolved (around line 196), apply the same defaulting before constructing `txDetail`:

```go
regular := input.Regular
if (txType == model.TxTypeIncome || txType == model.TxTypeExpense) && regular == nil {
    t := true
    regular = &t
}
// ...
txDetail := model.TransactionDetail{
    Timestamp:   input.Timestamp,
    Description: input.Description,
    Status:      input.Status,
    Type:        txType,
    Regular:     regular,
    Splits:      splits,
}
```

In `CreateTransactionFromSplits`, mirror the same pattern: copy `input.Regular` into `txDetail.Regular`.

In `UpdateTransactionComplete` (around line 280), after fetching `oldTx`:

```go
// If new Type is Income/Expense and caller didn't specify, default to &true.
// If new Type is not Income/Expense, force nil regardless of what the caller sent.
isApplicable := input.Type == model.TxTypeIncome || input.Type == model.TxTypeExpense
switch {
case isApplicable && input.Regular == nil:
    t := true
    input.Regular = &t
case !isApplicable:
    input.Regular = nil
}
if err := ValidateRegular(input.Type, input.Regular); err != nil {
    return err
}
```

And update the `UpdateTransactionBasic` call site to pass `input.Regular`:

```go
if err := repo.UpdateTransactionBasic(ctx, input.ID, input.Description, input.Timestamp, input.Status, input.Type, input.Regular); err != nil {
    return err
}
```

- [ ] **Step 4.7: Run the new tests**

Run: `go test ./internal/service/ -run "TestCreateTransaction_Regular|TestUpdateTransactionComplete_.*Regular" -v`
Expected: PASS (all four).

- [ ] **Step 4.8: Run the full service package to catch regressions**

Run: `go test ./internal/service/...`
Expected: PASS. (Pre-existing tests that construct `model.Transaction` literals without `Regular` still compile because `*bool` zero value is `nil`.)

- [ ] **Step 4.9: Commit**

```bash
git add internal/repository/interfaces.go internal/store/sqlite_transaction.go internal/service/transaction_ops.go internal/service/transaction_ops_test.go internal/service/testhelper_test.go
git commit -m "service: default and clear Regular through create/update"
```

---

## Task 5: Add migration 0011

**Files:**
- Create: `migrations/0011_add_transaction_regular.up.sql`
- Create: `migrations/0011_add_transaction_regular.down.sql`

- [ ] **Step 5.1: Write the up migration**

Create `migrations/0011_add_transaction_regular.up.sql`:

```sql
PRAGMA foreign_keys = OFF;

BEGIN;

CREATE TABLE transactions_new (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp   INTEGER NOT NULL,
    description TEXT,
    status      INTEGER NOT NULL DEFAULT 0,
    external_id TEXT,
    type        TEXT NOT NULL DEFAULT '',
    regular     INTEGER,
    CHECK (
        (type IN ('Income', 'Expense') AND regular IN (0, 1))
        OR (type NOT IN ('Income', 'Expense') AND regular IS NULL)
    )
);

INSERT INTO transactions_new (id, timestamp, description, status, external_id, type, regular)
SELECT id, timestamp, description, status, external_id, type,
       CASE WHEN type IN ('Income', 'Expense') THEN 1 ELSE NULL END
FROM transactions;

DROP TABLE transactions;
ALTER TABLE transactions_new RENAME TO transactions;

CREATE INDEX IF NOT EXISTS idx_transactions_timestamp ON transactions (timestamp);

COMMIT;

PRAGMA foreign_keys = ON;
```

Note: the previously-existing `external_id` UNIQUE constraint (added by migration 0002) must be reproduced. Verify migration 0002 first:

Run: `cat migrations/0002_add_external_id.up.sql`

If it adds a UNIQUE index (e.g., `CREATE UNIQUE INDEX idx_transactions_external_id ON transactions(external_id) WHERE external_id IS NOT NULL`), add the matching `CREATE UNIQUE INDEX ... IF NOT EXISTS` line at the bottom of the up migration before `COMMIT`. Match the exact name and condition from 0002.

- [ ] **Step 5.2: Write the down migration**

Create `migrations/0011_add_transaction_regular.down.sql`:

```sql
PRAGMA foreign_keys = OFF;

BEGIN;

CREATE TABLE transactions_old (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp   INTEGER NOT NULL,
    description TEXT,
    status      INTEGER NOT NULL DEFAULT 0,
    external_id TEXT,
    type        TEXT NOT NULL DEFAULT ''
);

INSERT INTO transactions_old (id, timestamp, description, status, external_id, type)
SELECT id, timestamp, description, status, external_id, type
FROM transactions;

DROP TABLE transactions;
ALTER TABLE transactions_old RENAME TO transactions;

CREATE INDEX IF NOT EXISTS idx_transactions_timestamp ON transactions (timestamp);

COMMIT;

PRAGMA foreign_keys = ON;
```

Also reproduce the external_id UNIQUE index here (same line as in up migration).

- [ ] **Step 5.3: Verify the migrations apply on a scratch DB**

Run: `go test ./internal/store/... -run TestSetupTestDB` — if no such test exists, just rely on the migration test in Task 6 to drive validation. Otherwise verify it doesn't break existing tests:

Run: `go test ./internal/store/...`
Expected: most tests will fail because store SQL has not been updated yet (Task 7). That is acceptable — what we are checking here is that the migrations themselves apply cleanly (no SQL syntax errors). If you see "syntax error", fix the migration. If you see "constraint failed" / "no such column", continue.

- [ ] **Step 5.4: Commit**

```bash
git add migrations/0011_add_transaction_regular.up.sql migrations/0011_add_transaction_regular.down.sql
git commit -m "migration: add regular column with CHECK invariant"
```

---

## Task 6: Migration 0011 round-trip test

**Files:**
- Create: `internal/store/migration_0011_test.go`

- [ ] **Step 6.1: Write the test**

Create `internal/store/migration_0011_test.go` modeled on `migration_0010_test.go`:

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package store_test

import (
    "context"
    "testing"

    "github.com/hance08/kea/internal/model"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestMigration0011_BackfillRegular(t *testing.T) {
    s := setupTestDB(t)
    ctx := context.Background()

    // Insert accounts.
    bankID, err := s.CreateAccount(ctx, "Assets:Bank:Main", model.AccountTypeAsset, "USD", "", nil)
    require.NoError(t, err)
    foodID, err := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
    require.NoError(t, err)
    salaryID, err := s.CreateAccount(ctx, "Revenue:Salary", model.AccountTypeRevenue, "USD", "", nil)
    require.NoError(t, err)

    boolPtr := func(b bool) *bool { return &b }

    // Income transaction — must come out regular=1.
    incomeID, err := s.CreateTransactionWithSplits(ctx, model.Transaction{
        Timestamp: 1_700_000_000, Description: "Salary",
        Status: model.StatusCleared, Type: model.TxTypeIncome,
        Regular: boolPtr(true),
    }, []model.Split{
        {AccountID: bankID, Amount: 100_000, Currency: "USD"},
        {AccountID: salaryID, Amount: -100_000, Currency: "USD"},
    })
    require.NoError(t, err)

    // Expense transaction — must come out regular=1.
    expenseID, err := s.CreateTransactionWithSplits(ctx, model.Transaction{
        Timestamp: 1_700_000_001, Description: "Lunch",
        Status: model.StatusCleared, Type: model.TxTypeExpense,
        Regular: boolPtr(true),
    }, []model.Split{
        {AccountID: foodID, Amount: 500, Currency: "USD"},
        {AccountID: bankID, Amount: -500, Currency: "USD"},
    })
    require.NoError(t, err)

    // Transfer transaction — must come out regular=NULL.
    transferID, err := s.CreateTransactionWithSplits(ctx, model.Transaction{
        Timestamp: 1_700_000_002, Description: "ATM",
        Status: model.StatusCleared, Type: model.TxTypeTransfer,
    }, []model.Split{
        {AccountID: bankID, Amount: -1_000, Currency: "USD"},
        {AccountID: bankID, Amount: 1_000, Currency: "USD"},
    })
    require.NoError(t, err)

    fetchRegular := func(id int64) *int64 {
        var v *int64
        require.NoError(t, s.DB().QueryRowContext(ctx,
            `SELECT regular FROM transactions WHERE id = ?`, id).Scan(&v))
        return v
    }
    deref := func(p *int64) int64 {
        require.NotNil(t, p)
        return *p
    }

    assert.Equal(t, int64(1), deref(fetchRegular(incomeID)), "Income should backfill to regular=1")
    assert.Equal(t, int64(1), deref(fetchRegular(expenseID)), "Expense should backfill to regular=1")
    assert.Nil(t, fetchRegular(transferID), "Transfer should backfill to regular=NULL")
}

func TestMigration0011_CheckConstraint(t *testing.T) {
    s := setupTestDB(t)
    ctx := context.Background()
    db := s.DB()

    // Direct SQL inserts that violate the invariant.
    _, err := db.ExecContext(ctx,
        `INSERT INTO transactions (timestamp, description, status, type, regular) VALUES (?, ?, ?, ?, ?)`,
        1, "x", 0, "Income", nil)
    assert.Error(t, err, "Income with regular=NULL must violate CHECK")

    _, err = db.ExecContext(ctx,
        `INSERT INTO transactions (timestamp, description, status, type, regular) VALUES (?, ?, ?, ?, ?)`,
        1, "x", 0, "Transfer", 1)
    assert.Error(t, err, "Transfer with regular=1 must violate CHECK")

    _, err = db.ExecContext(ctx,
        `INSERT INTO transactions (timestamp, description, status, type, regular) VALUES (?, ?, ?, ?, ?)`,
        1, "ok", 0, "Income", 0)
    assert.NoError(t, err, "Income with regular=0 must succeed")
}
```

(The `regular` column is `INTEGER`; SQLite scans NULL into `*int64` as nil, and stores Go `*bool(true)` as 1 / `*bool(false)` as 0 once Task 7 wires the store. Until then this test depends on Task 7's store changes, so it will fail at Step 6.2 — that is expected.)

- [ ] **Step 6.2: Run the test to verify it fails**

Run: `go test ./internal/store/ -run TestMigration0011 -v`
Expected: failures — the store SQL still SELECTs/INSERTs without `regular`, so reads/writes will fail or return wrong data. Continue.

- [ ] **Step 6.3: Commit**

```bash
git add internal/store/migration_0011_test.go
git commit -m "store: add migration 0011 round-trip and CHECK constraint tests"
```

---

## Task 7: Extend SQLite store for the `regular` column

**Files:**
- Modify: `internal/store/sqlite_transaction.go`

The `regular` column must be threaded through every query that touches `transactions`. Below is the exhaustive list; modify each query in place.

- [ ] **Step 7.1: `CreateTransactionWithSplits` (around line 20)**

```go
stmtTx, err := s.db.PrepareContext(ctx, `
    INSERT INTO transactions (timestamp, description, status, external_id, type, regular)
    VALUES (?, ?, ?, ?, ?, ?)
    RETURNING id;
`)
// ...
err = stmtTx.QueryRowContext(ctx, tx.Timestamp, tx.Description, tx.Status, tx.ExternalID, tx.Type, tx.Regular).Scan(&newTxID)
```

`*bool` binds to SQLite as NULL / 0 / 1 automatically via `database/sql`.

- [ ] **Step 7.2: `GetTransactionByID` (around line 70)**

```go
err := s.db.QueryRowContext(ctx, `
    SELECT id, timestamp, description, status, external_id, type, regular
    FROM transactions
    WHERE id = ?
`, txID).Scan(&tx.ID, &tx.Timestamp, &tx.Description, &tx.Status, &tx.ExternalID, &tx.Type, &tx.Regular)
```

- [ ] **Step 7.3: `GetTransactionsByAccount` (around line 89)**

Update `SELECT DISTINCT t.id, t.timestamp, t.description, t.status, t.external_id, t.type` to include `, t.regular`.

- [ ] **Step 7.4: `GetTransactionsByDateRange` (around line 115)**

Update the SELECT list to include `regular`.

- [ ] **Step 7.5: `GetAllTransactions` (around line 135)**

Same — add `regular`.

- [ ] **Step 7.6: `UpdateTransactionBasic` (around line 208)**

```go
result, err := s.db.ExecContext(ctx, `
    UPDATE transactions
    SET description = ?, timestamp = ?, status = ?, type = ?, regular = ?
    WHERE id = ?
`, description, timestamp, status, txType, regular, txID)
```

Remove the placeholder `_ = regular` line added in Task 4.

- [ ] **Step 7.7: Any other `SELECT … FROM transactions` queries**

Search for unmissed queries:

Run: `grep -n "FROM transactions" internal/store/sqlite_transaction.go`

For each remaining match (e.g., the queries around line 498, 542, 647), append `, regular` (or `, t.regular` when aliased) to the SELECT list.

- [ ] **Step 7.8: `scanTransactions` (around line 565)**

```go
err := rows.Scan(&tx.ID, &tx.Timestamp, &tx.Description, &tx.Status, &tx.ExternalID, &tx.Type, &tx.Regular)
```

- [ ] **Step 7.9: `FilterTransactions` filter clause (around line 583)**

After the existing `if filter.Description != nil { ... }` block, append:

```go
if filter.Regular != nil {
    whereClauses = append(whereClauses, "t.regular = ?")
    if *filter.Regular {
        args = append(args, 1)
    } else {
        args = append(args, 0)
    }
}
```

NULL rows are naturally excluded by `t.regular = ?` (NULL comparisons are unknown), which matches the spec.

Also update the SELECT in the `fmt.Sprintf` literal (around line 647) to include `t.regular`:

```go
query := fmt.Sprintf(
    "SELECT %st.id, t.timestamp, t.description, t.status, t.external_id, t.type, t.regular %s %s ORDER BY t.timestamp DESC, t.id DESC LIMIT ? OFFSET ?",
    selectDistinct, fromSQL, whereSQL,
)
```

- [ ] **Step 7.10: Run the migration tests**

Run: `go test ./internal/store/ -run TestMigration0011 -v`
Expected: PASS.

- [ ] **Step 7.11: Run all store tests**

Run: `go test ./internal/store/...`
Expected: PASS. (Any failures here are missed SELECT lists; grep again for `FROM transactions` and add `regular` to the column list.)

- [ ] **Step 7.12: Run the full project**

Run: `go test ./...`
Expected: PASS, except possibly the SPA and a few API tests we'll cover in later tasks. All Go service / store / model tests must pass.

- [ ] **Step 7.13: Commit**

```bash
git add internal/store/sqlite_transaction.go
git commit -m "store: persist and query Transaction.Regular column"
```

---

## Task 8: Extend `FilterTransactions` filter test

**Files:**
- Modify: `internal/store/sqlite_transaction_filter_test.go`

- [ ] **Step 8.1: Add a filter-by-regular test**

Append to `internal/store/sqlite_transaction_filter_test.go`:

```go
func TestFilterTransactions_ByRegular(t *testing.T) {
    s := setupTestDB(t)
    ctx := context.Background()

    bankID, _ := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
    foodID, _ := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
    salaryID, _ := s.CreateAccount(ctx, "Revenue:Salary", model.AccountTypeRevenue, "USD", "", nil)

    boolPtr := func(b bool) *bool { return &b }

    // Regular expense.
    _, err := s.CreateTransactionWithSplits(ctx, model.Transaction{
        Timestamp: 1, Description: "rent", Status: model.StatusCleared,
        Type: model.TxTypeExpense, Regular: boolPtr(true),
    }, []model.Split{
        {AccountID: foodID, Amount: 1_000, Currency: "USD"},
        {AccountID: bankID, Amount: -1_000, Currency: "USD"},
    })
    require.NoError(t, err)

    // Irregular expense.
    _, err = s.CreateTransactionWithSplits(ctx, model.Transaction{
        Timestamp: 2, Description: "vacation", Status: model.StatusCleared,
        Type: model.TxTypeExpense, Regular: boolPtr(false),
    }, []model.Split{
        {AccountID: foodID, Amount: 2_000, Currency: "USD"},
        {AccountID: bankID, Amount: -2_000, Currency: "USD"},
    })
    require.NoError(t, err)

    // Transfer (regular=NULL).
    _, err = s.CreateTransactionWithSplits(ctx, model.Transaction{
        Timestamp: 3, Description: "ATM", Status: model.StatusCleared,
        Type: model.TxTypeTransfer,
    }, []model.Split{
        {AccountID: bankID, Amount: -500, Currency: "USD"},
        {AccountID: salaryID, Amount: 500, Currency: "USD"},
    })
    require.NoError(t, err)

    onlyReg := s.mustFilter(t, ctx, model.TransactionFilter{Regular: boolPtr(true)})
    assert.Equal(t, 1, len(onlyReg.Items), "only the regular expense should match Regular=true")

    onlyIrreg := s.mustFilter(t, ctx, model.TransactionFilter{Regular: boolPtr(false)})
    assert.Equal(t, 1, len(onlyIrreg.Items), "only the irregular expense should match Regular=false")

    noFilter := s.mustFilter(t, ctx, model.TransactionFilter{})
    assert.Equal(t, 3, len(noFilter.Items), "no filter returns all three")
}
```

If `mustFilter` doesn't exist, inline the call: `s.FilterTransactions(ctx, filter, model.ListOptions{Limit: 100})`.

- [ ] **Step 8.2: Run**

Run: `go test ./internal/store/ -run TestFilterTransactions_ByRegular -v`
Expected: PASS.

- [ ] **Step 8.3: Commit**

```bash
git add internal/store/sqlite_transaction_filter_test.go
git commit -m "store: test filter by Regular"
```

---

## Task 9: Service `BuildTransactionListItems` carries `Regular`

**Files:**
- Modify: `internal/service/transaction_classifier.go`
- Test: `internal/service/transaction_classifier_test.go`

- [ ] **Step 9.1: Find `BuildTransactionListItems` and add `Regular`**

Open `internal/service/transaction_classifier.go` and locate `BuildTransactionListItems`. Inside the per-transaction loop where it builds the `TransactionListItem`, add `Regular: tx.Regular` to the struct literal.

- [ ] **Step 9.2: Add a test case**

Append a sub-test (or extend an existing test) in `transaction_classifier_test.go` that asserts a built `TransactionListItem.Regular` equals the source transaction's `Regular`.

- [ ] **Step 9.3: Run**

Run: `go test ./internal/service/ -run TestBuildTransactionListItems -v`
Expected: PASS.

- [ ] **Step 9.4: Commit**

```bash
git add internal/service/transaction_classifier.go internal/service/transaction_classifier_test.go
git commit -m "service: propagate Regular through TransactionListItem"
```

---

## Task 10: API parses `regular` query parameter

**Files:**
- Modify: `internal/api/params.go`
- Test: `internal/api/params_test.go`

- [ ] **Step 10.1: Write the failing test**

Append to `internal/api/params_test.go`:

```go
func TestParseTransactionFilter_Regular(t *testing.T) {
    t.Run("regular=true", func(t *testing.T) {
        f, err := parseTransactionFilter(reqWithQuery(t, "regular=true"))
        require.NoError(t, err)
        require.NotNil(t, f.Regular)
        assert.True(t, *f.Regular)
    })
    t.Run("regular=false", func(t *testing.T) {
        f, err := parseTransactionFilter(reqWithQuery(t, "regular=false"))
        require.NoError(t, err)
        require.NotNil(t, f.Regular)
        assert.False(t, *f.Regular)
    })
    t.Run("regular=0/1 also accepted", func(t *testing.T) {
        f, err := parseTransactionFilter(reqWithQuery(t, "regular=1"))
        require.NoError(t, err)
        require.NotNil(t, f.Regular)
        assert.True(t, *f.Regular)
    })
    t.Run("missing -> nil", func(t *testing.T) {
        f, err := parseTransactionFilter(reqWithQuery(t, ""))
        require.NoError(t, err)
        assert.Nil(t, f.Regular)
    })
    t.Run("invalid -> error", func(t *testing.T) {
        _, err := parseTransactionFilter(reqWithQuery(t, "regular=maybe"))
        assert.Error(t, err)
    })
}
```

- [ ] **Step 10.2: Run to verify it fails**

Run: `go test ./internal/api/ -run TestParseTransactionFilter_Regular -v`
Expected: failure — `f.Regular` is always nil.

- [ ] **Step 10.3: Implement**

In `internal/api/params.go`, inside `parseTransactionFilter`, after the `f.Description = parseStringQuery(r, "description")` line, before the `return f, nil`:

```go
if raw := r.URL.Query().Get("regular"); raw != "" {
    switch strings.ToLower(raw) {
    case "true", "1":
        t := true
        f.Regular = &t
    case "false", "0":
        v := false
        f.Regular = &v
    default:
        return f, &service.ValidationError{
            Field:   "regular",
            Message: "regular must be true/false/1/0",
        }
    }
}
```

- [ ] **Step 10.4: Run**

Run: `go test ./internal/api/ -run TestParseTransactionFilter_Regular -v`
Expected: PASS.

- [ ] **Step 10.5: Commit**

```bash
git add internal/api/params.go internal/api/params_test.go
git commit -m "api: parse ?regular=true|false for transaction filter"
```

---

## Task 11: API write endpoints round-trip `Regular`

**Files:**
- Modify: `internal/api/transactions_write.go` (only if it currently strips unknown fields — likely a no-op since it deserializes into `model.UpdateTransactionInput` / `model.CreateTransactionFromSplitsInput` which Task 1 already extended).
- Test: `internal/api/transactions_write_test.go`

- [ ] **Step 11.1: Read the current write handler**

Run: `grep -n "UpdateTransaction\|CreateTransaction\|Decode" internal/api/transactions_write.go | head`

The handlers likely already json-decode into the model input types. Confirm the new `Regular` field is decoded by reading the handler around the `json.NewDecoder(r.Body).Decode(&input)` line.

- [ ] **Step 11.2: Add a round-trip test**

Append to `internal/api/transactions_write_test.go`:

```go
func TestCreateTransaction_RoundTripRegular(t *testing.T) {
    // Set up server with mock service or in-memory store per project pattern.
    // Modeled on existing tests in transactions_write_test.go.
    // ...arrange...
    body := `{
      "description": "Coffee",
      "timestamp": 1700000000,
      "status": "Cleared",
      "type": "Expense",
      "regular": false,
      "splits": [
        {"account_name":"Expenses:Coffee","amount":150,"currency":"USD"},
        {"account_name":"Assets:Cash","amount":-150,"currency":"USD"}
      ]
    }`
    // POST body, decode response, assert response.Regular == &false.
}

func TestUpdateTransaction_RoundTripRegular(t *testing.T) {
    // Seed an Expense with Regular=true; PUT with regular=false; GET; assert =&false.
}
```

Fill in arrange/act/assert by mirroring the closest existing test in the same file. If the file is large, add a comment pointing the engineer at the closest existing helper.

- [ ] **Step 11.3: Run**

Run: `go test ./internal/api/ -run "TestCreateTransaction_RoundTripRegular|TestUpdateTransaction_RoundTripRegular" -v`
Expected: PASS.

- [ ] **Step 11.4: Commit**

```bash
git add internal/api/transactions_write_test.go
git commit -m "api: round-trip Regular through create/update endpoints"
```

---

## Task 12: CLI `kea add` — `--regular` flag and interactive prompt

**Files:**
- Modify: `cmd/add.go`
- Modify: `cmd/add_types.go`
- Modify: `cmd/add_actions.go`

- [ ] **Step 12.1: Add a tri-state flag field**

In `cmd/add_types.go`, add to the `addFlags` struct:

```go
RegularSet bool // user passed --regular or --no-regular
Regular    bool // the value
```

Cobra has no native tri-state. We use a paired-flag pattern. In `cmd/add.go` `NewAddCmd`, add:

```go
cmd.Flags().BoolVar(&flags.Regular, "regular", true, "Mark as regular (default true; only honored for Income/Expense)")
cmd.Flags().BoolVar(&flags.Regular, "no-regular", false, "Mark as irregular")
// After Flags() definitions, in RunE before calling runner.Run, set:
// flags.RegularSet = cmd.Flags().Changed("regular") || cmd.Flags().Changed("no-regular")
```

Actually, a cleaner pattern: register one flag, and detect "user passed it" via `cmd.Flags().Changed("regular")`. Use this instead:

```go
cmd.Flags().Bool("regular", true, "Mark Income/Expense as regular (use --regular=false for irregular)")
```

Then in `RunE`, just before invoking `runner.Run`:

```go
if cmd.Flags().Changed("regular") {
    flags.RegularSet = true
    flags.Regular, _ = cmd.Flags().GetBool("regular")
}
```

Remove `--no-regular` — keep the surface minimal (`--regular=false` works fine for cobra).

- [ ] **Step 12.2: Plumb the flag into `addTransactionInput`**

Open `cmd/add_actions.go` and find `runFromFlags`. Just before constructing the input that gets returned, compute:

```go
var regular *bool
if flags.RegularSet {
    isApplicable := input.Type == model.TxTypeIncome || input.Type == model.TxTypeExpense
    if !isApplicable {
        fmt.Fprintf(os.Stderr, "warning: --regular is only honored for Income/Expense; ignored for %s\n", input.Type)
    } else {
        v := flags.Regular
        regular = &v
    }
}
input.Regular = regular
```

(Add `Regular *bool` to `addTransactionInput` in `cmd/add_types.go` first if it isn't there.)

Then update both `CreateSimpleTransaction` and `CreateTransactionFromSplits` call sites in `cmd/add.go` `Run` to pass `Regular: input.Regular`:

```go
result, err = r.txSvc.CreateSimpleTransaction(
    ctx, model.CreateSimpleTransactionInput{
        // existing fields ...
        Regular: input.Regular,
    },
)
// and:
result, err = r.txSvc.CreateTransactionFromSplits(
    ctx, model.CreateTransactionFromSplitsInput{
        // existing fields ...
        Regular: input.Regular,
    },
)
```

- [ ] **Step 12.3: Add the interactive prompt**

In `cmd/add_actions.go` `runInteractive`, after the Type prompt resolves and `input.Type` is set, append:

```go
if input.Type == model.TxTypeIncome || input.Type == model.TxTypeExpense {
    var regular bool = true
    if err := huh.NewSelect[bool]().
        Title("Regular spending?").
        Description("Tick if this is a habitual income/expense (e.g. salary, rent).").
        Options(
            huh.NewOption("Yes (regular)", true).Selected(true),
            huh.NewOption("No (one-off)", false),
        ).
        Value(&regular).
        Run(); err != nil {
        return addTransactionInput{}, err
    }
    input.Regular = &regular
}
```

Add the `huh` import (`github.com/charmbracelet/huh`) if not present.

- [ ] **Step 12.4: Update existing add tests if they reference `addTransactionInput` literally**

Run: `go test ./cmd/...`
Expected: PASS (`Regular *bool` zero-value is nil, so existing test literals still compile).

- [ ] **Step 12.5: Commit**

```bash
git add cmd/add.go cmd/add_types.go cmd/add_actions.go
git commit -m "cmd: add --regular flag and interactive prompt to kea add"
```

---

## Task 13: CLI `kea transaction edit` — toggle and type-change side effect

**Files:**
- Modify: `cmd/transaction/edit_types.go`
- Modify: `cmd/transaction/edit.go`
- Modify: `cmd/transaction/edit_actions.go`

- [ ] **Step 13.1: Add the menu option constant**

In `cmd/transaction/edit_types.go`, alongside the existing `Opt…` constants:

```go
OptToggleRegular = "Toggle Regular"
```

- [ ] **Step 13.2: Register the menu entry**

In `cmd/transaction/edit.go`, locate the slice of `editOption`s. Add:

```go
{
    Label:     OptToggleRegular,
    Condition: func(d *model.TransactionDetail) bool {
        return d.Type == model.TxTypeIncome || d.Type == model.TxTypeExpense
    },
    Action: func(d *model.TransactionDetail) error {
        return r.actionToggleRegular(ctx, d)
    },
},
```

Match the closure/signature pattern used by adjacent options in the file.

- [ ] **Step 13.3: Implement `actionToggleRegular`**

In `cmd/transaction/edit_actions.go`:

```go
func (r *editRunner) actionToggleRegular(ctx context.Context, d *model.TransactionDetail) error {
    if d.Regular == nil {
        return fmt.Errorf("regular attribute is not set on this transaction")
    }
    flipped := !*d.Regular
    d.Regular = &flipped
    return r.actionSave(ctx, d)
}
```

If the closest existing action does not call `actionSave` directly, mirror that pattern (some editors collect changes and save explicitly via the Save menu option).

- [ ] **Step 13.4: Update `actionEditType` to maintain the invariant**

Find `actionEditType` (around line 257 of `edit_actions.go`). After the new type is captured into `detail.Type`, before saving:

```go
isApplicable := detail.Type == model.TxTypeIncome || detail.Type == model.TxTypeExpense
switch {
case isApplicable && detail.Regular == nil:
    t := true
    detail.Regular = &t
case !isApplicable:
    detail.Regular = nil
}
```

- [ ] **Step 13.5: Run**

Run: `go test ./cmd/transaction/...`
Expected: PASS. (The service layer also re-applies the same rule, so even if a code path forgets, validation catches it.)

- [ ] **Step 13.6: Commit**

```bash
git add cmd/transaction/edit_types.go cmd/transaction/edit.go cmd/transaction/edit_actions.go
git commit -m "cmd: edit can toggle Regular and clears it on type change"
```

---

## Task 14: CLI `kea transaction list` — filter flag and `Reg` column

**Files:**
- Modify: `cmd/transaction/list.go`
- Modify: `ui/views/...` (the file that renders the list table — locate via grep)

- [ ] **Step 14.1: Add filter flag**

In `cmd/transaction/list.go`, in the flags struct:

```go
RegularSet bool
Regular    bool
```

In the `Flags().…` block:

```go
cmd.Flags().Bool("regular", true, "Filter by regular flag (use --regular=false for irregular)")
```

In `RunE`, before `runner.Run`:

```go
if cmd.Flags().Changed("regular") {
    flags.RegularSet = true
    flags.Regular, _ = cmd.Flags().GetBool("regular")
}
```

In `fetchTransactions`, when building the `TransactionFilter`, include:

```go
if r.flags.RegularSet {
    v := r.flags.Regular
    filter.Regular = &v
}
```

- [ ] **Step 14.2: Add the `Reg` column to the view**

Locate the list table renderer:

Run: `grep -rn "TransactionListItem\|Status\b" ui/views/ | head`

In the file that renders `TransactionListItem` (likely `ui/views/transaction_list.go` or similar), add a `Reg` column header and render:

```go
func regCell(r *bool) string {
    if r == nil {
        return ""
    }
    if *r {
        return "✓"
    }
    return "✗"
}
```

Append the cell to the row construction so it lines up under the new column header.

- [ ] **Step 14.3: Run**

Run: `go test ./cmd/transaction/... && go build ./...`
Expected: PASS.

- [ ] **Step 14.4: Commit**

```bash
git add cmd/transaction/list.go ui/views/
git commit -m "cmd: list supports --regular filter and shows Reg column"
```

---

## Task 15: CLI `kea report` — filter by `Regular`

**Files:**
- Modify: `cmd/report.go`
- Modify: `cmd/report_actions.go` (if filter is built there)

- [ ] **Step 15.1: Add the flag and plumb to filter**

Mirror Task 14's pattern: register a `--regular` flag, detect `Changed`, and propagate `*bool` into the report's `TransactionFilter`.

- [ ] **Step 15.2: Run**

Run: `go test ./cmd/... && go build ./...`
Expected: PASS.

- [ ] **Step 15.3: Commit**

```bash
git add cmd/report.go cmd/report_actions.go
git commit -m "cmd: report supports --regular filter"
```

---

## Task 16: SPA types and FilterBar tri-state

**Files:**
- Modify: `spa/src/lib/types.ts`
- Modify: `spa/src/components/transactions/FilterBar.tsx`

- [ ] **Step 16.1: Extend types**

In `spa/src/lib/types.ts`, find the `Transaction` and `TransactionFilter` interfaces and add:

```ts
export interface Transaction {
  // existing fields …
  regular?: boolean;          // present only for Income/Expense
}

export interface TransactionFilter {
  // existing fields …
  regular?: boolean;          // undefined = "Any"
}
```

If a `TransactionInput` (form payload) type exists, add `regular?: boolean` there too.

- [ ] **Step 16.2: Add the tri-state select to FilterBar**

In `spa/src/components/transactions/FilterBar.tsx`, add a new control next to the existing Type/Status filters:

```tsx
<label>
  Regular
  <select
    value={filter.regular === undefined ? "" : filter.regular ? "true" : "false"}
    onChange={(e) => {
      const v = e.target.value;
      onChange({ ...filter, regular: v === "" ? undefined : v === "true" });
    }}
  >
    <option value="">Any</option>
    <option value="true">Regular only</option>
    <option value="false">Irregular only</option>
  </select>
</label>
```

Match the existing component's styling/className conventions (look at the Type filter for reference).

In the URL serialization code (look for where `type` or `status` is written into the query string), append `regular=true|false` when set.

- [ ] **Step 16.3: Run SPA tests**

Run: `cd spa && npm test -- --run`
Expected: PASS. (Some snapshots may need updating — `npm test -- -u` for those.)

- [ ] **Step 16.4: Commit**

```bash
git add spa/src/lib/types.ts spa/src/components/transactions/FilterBar.tsx
git commit -m "spa: add Regular tri-state to FilterBar and types"
```

---

## Task 17: SPA TransactionRow badge

**Files:**
- Modify: `spa/src/components/transactions/TransactionRow.tsx`
- Modify: `spa/src/test/transactions.list.test.tsx`

- [ ] **Step 17.1: Add the badge**

In `TransactionRow.tsx`, near where Type or Status badges render, add:

```tsx
{tx.regular === true && (
  <span className="badge badge-regular" title="Regular">R</span>
)}
{tx.regular === false && (
  <span className="badge badge-irregular" title="One-off">1×</span>
)}
```

Style with whatever class/tailwind utility matches surrounding badges. If badges in this codebase live in a shared component, follow that pattern.

- [ ] **Step 17.2: Test row rendering**

Append to `spa/src/test/transactions.list.test.tsx`:

```tsx
it("renders regular badge for regular expense", () => {
  // Render the list with a fixture expense having regular: true.
  // Assert that the row contains the "R" badge.
});

it("does not render badge for transfer (no regular)", () => {
  // Render with a transfer transaction.
  // Assert no Regular badge is present.
});
```

Inline the test bodies using the existing helpers in the file (`renderList`, fixture factories, etc.).

- [ ] **Step 17.3: Run**

Run: `cd spa && npm test -- --run transactions.list`
Expected: PASS.

- [ ] **Step 17.4: Commit**

```bash
git add spa/src/components/transactions/TransactionRow.tsx spa/src/test/transactions.list.test.tsx
git commit -m "spa: render Regular badge in transaction rows"
```

---

## Task 18: SPA form — Regular checkbox + conditional + type-change side effect

**Files:**
- Modify: `spa/src/components/transactions/SimpleFields.tsx`
- Modify: `spa/src/components/transactions/TransactionForm.tsx`
- Create: `spa/src/test/transactions.regular.test.tsx`

- [ ] **Step 18.1: Add the checkbox to `SimpleFields.tsx`**

Find where the Type select renders. Append, gated by type:

```tsx
{(form.type === "Income" || form.type === "Expense") && (
  <label className="flex items-center gap-2">
    <input
      type="checkbox"
      checked={form.regular ?? true}
      onChange={(e) => onChange({ ...form, regular: e.target.checked })}
    />
    Regular
    <span className="text-xs text-muted">
      Tick if this is a habitual income/expense (e.g. salary, rent).
    </span>
  </label>
)}
```

- [ ] **Step 18.2: Handle type-change side effect**

In `TransactionForm.tsx` (or wherever the Type onChange lives), when the user changes type:

```ts
const isApplicable = (t: TransactionType) => t === "Income" || t === "Expense";
function onTypeChange(t: TransactionType) {
  setForm((f) => ({
    ...f,
    type: t,
    regular: isApplicable(t) ? (f.regular ?? true) : undefined,
  }));
}
```

This mirrors the Go service rule: leaving Income/Expense clears `regular`; entering Income/Expense defaults to `true`.

- [ ] **Step 18.3: Write the form test**

Create `spa/src/test/transactions.regular.test.tsx`:

```tsx
import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect } from "vitest";
// Import the form / SimpleFields and any test harness.

describe("Transaction form Regular field", () => {
  it("shows the Regular checkbox for Expense", () => {
    // render with form.type = "Expense"
    expect(screen.getByLabelText(/Regular/i)).toBeInTheDocument();
  });

  it("hides the Regular checkbox for Transfer", () => {
    // render with form.type = "Transfer"
    expect(screen.queryByLabelText(/Regular/i)).not.toBeInTheDocument();
  });

  it("clears regular when type changes to Transfer", () => {
    // render with Expense + regular=true, then change type to Transfer
    // assert the form's regular value is undefined (use a test harness that
    // exposes form state, or assert the checkbox is no longer rendered).
  });
});
```

Use the same render harness pattern as the other SPA tests in `spa/src/test/`.

- [ ] **Step 18.4: Run**

Run: `cd spa && npm test -- --run`
Expected: PASS.

- [ ] **Step 18.5: Commit**

```bash
git add spa/src/components/transactions/SimpleFields.tsx spa/src/components/transactions/TransactionForm.tsx spa/src/test/transactions.regular.test.tsx
git commit -m "spa: form Regular checkbox with conditional rendering"
```

---

## Task 19: Rebuild SPA bundle and commit

**Files:**
- Modify: `internal/web/dist/*`

- [ ] **Step 19.1: Build the SPA**

Run: `cd spa && npm run build`
Expected: build succeeds; output written to `internal/web/dist/`.

- [ ] **Step 19.2: Verify the Go embed picks up the new bundle**

Run: `go build ./...`
Expected: success.

- [ ] **Step 19.3: Quick smoke test the server**

Run: `go run ./cmd/kea serve --addr :8765 &` (background), open the SPA in a browser, verify the FilterBar has a Regular tri-state and an Income/Expense form shows the checkbox. Stop the server.

If you cannot start a server in your environment, run: `go test ./internal/web/... ./internal/api/...` instead and rely on the integration coverage there.

- [ ] **Step 19.4: Commit the refreshed bundle**

```bash
git add internal/web/dist/
git commit -m "build(spa): refresh embedded bundle for Regular attribute"
```

---

## Task 20: Final full-project test sweep

- [ ] **Step 20.1: Run everything**

Run: `go test ./... && (cd spa && npm test -- --run)`
Expected: PASS.

- [ ] **Step 20.2: Tidy and verify build**

Run: `go mod tidy && go build ./...`
Expected: PASS.

- [ ] **Step 20.3: Final commit if `go.sum` changed**

```bash
git add go.mod go.sum
git diff --cached --quiet || git commit -m "chore: go mod tidy"
```

---

## Self-Review Notes

- All spec sections are covered: Section 1 (data model) → Tasks 1, 5, 6; Section 2 (repo/service) → Tasks 2–4, 7–9; Section 3 (UI) → Tasks 12–18; Section 4 (migration / rollout / tests) → Tasks 5–6, 10–11, 19–20.
- Default rule (`true` for new Income/Expense) is enforced in three places, all aligned: service create paths (Task 4), CLI interactive prompt default (Task 12), SPA form default (Task 18). DB has no default so the service layer is the single source of truth for "new" rows.
- Type-change clearing/defaulting appears in both the CLI edit (Task 13) and SPA form (Task 18); the service layer also enforces it (Task 4) so even API callers that forget are corrected.
- The `UpdateTransactionBasic` repository signature change touches the interface, the SQLite implementation, the mock, and the service caller — all enumerated in Task 4.
- Migration 0011 preserves the `external_id` UNIQUE index from migration 0002 (Step 5.1 includes a verify-and-mirror step).
- SQL changes in Task 7 enumerate every `FROM transactions` site in `sqlite_transaction.go` so none are missed.
