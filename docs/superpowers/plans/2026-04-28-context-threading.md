# Context Threading Refactor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Thread `context.Context` as the first parameter through every DB-touching method across the store, repository interface, service, and cmd layers so queries can be cancelled and time-bounded.

**Architecture:** Bottom-up mechanical sweep: DBTX interface → repository interfaces → store implementations → service test mocks → service methods → cmd interfaces → cmd call sites. Tasks 1 compiles cleanly in isolation; Tasks 2–10 are interdependent and must all complete before the build passes again. Commit after Task 1 (safe checkpoint), then commit again after the full sweep.

**Tech Stack:** Go standard library `context.Context`, `database/sql` `*Context` variants (`ExecContext`, `QueryContext`, `QueryRowContext`, `PrepareContext`), Cobra (`cmd.Context()`).

---

## File Map

Modified files and what changes in each:

| File | Change |
|------|--------|
| `internal/store/sqlite.go` | `DBTX` interface → `*Context` methods |
| `internal/repository/interfaces.go` | Add `ctx context.Context` first param to all 32 methods (13 AccountRepository + 19 TransactionRepository) |
| `internal/store/sqlite_account.go` | Add ctx to 13 method signatures + use `*Context` DB calls |
| `internal/store/sqlite_transaction.go` | Add ctx to ~15 method signatures + use `*Context` DB calls |
| `internal/store/sqlite_reconcile.go` | Add ctx to 5 method signatures + use `*Context` DB calls |
| `internal/service/testhelper_test.go` | Add `_ context.Context` first param to all mock methods |
| `internal/service/account_service.go` | Add ctx to 8 methods; pass ctx to repo calls |
| `internal/service/account_ops.go` | Add ctx to 5 methods; `ExecTx(ctx,` replaces `ExecTx(context.Background(),` |
| `internal/service/transaction_service.go` | Add ctx to 5 methods |
| `internal/service/transaction_ops.go` | Add ctx to 6 methods; replace hardcoded `context.Background()` |
| `internal/service/transaction_validation.go` | Add ctx to `ValidateTransactionEdit` (calls accRepo) |
| `internal/service/reconcile_ops.go` | Add ctx to 2 methods; replace hardcoded `context.Background()` |
| `internal/service/report_service.go` | Add ctx to 5 methods (incl. private `buildReportMaps`) |
| `cmd/add_types.go` | Add ctx to `AddProvider` + `TransactionProvider` methods |
| `cmd/report_types.go` | Add ctx to `ReportProvider` methods |
| `cmd/reconcile.go` | Add ctx to `reconcileAccountProvider` + `reconcileTxProvider` methods |
| `cmd/transaction/list.go` | Add ctx to `ListProvider` methods |
| `cmd/transaction/edit_types.go` | Add ctx to `EditProvider` + `AccountProvider` DB-touching methods |
| `cmd/transaction/clear.go` | Add ctx to `ClearProvider` |
| `cmd/transaction/show.go` | Add ctx to `ShowProvider` |
| `cmd/transaction/delete.go` | Add ctx to `TxDeleteProvider` |
| `cmd/account/list.go` | Add ctx to `AccountListProvider` methods |
| `cmd/account/delete.go` | Add ctx to `AccountDeleteProvider` |
| `cmd/account/edit_types.go` | Add ctx to `EditProvider` DB-touching methods |
| `cmd/account/create_types.go` | Add ctx to `CreateProvider` DB-touching methods |
| `cmd/add.go` | Pass ctx in `Run` |
| `cmd/add_actions.go` | Pass ctx down through action helpers |
| `cmd/add_test.go` | Add `_ context.Context` to mock method signatures |
| `cmd/report.go` | Pass `cmd.Context()` to `runner.run()` |
| `cmd/report_actions.go` | Accept + thread ctx through report runners |
| `cmd/reconcile_actions.go` | Thread ctx through reconcile runners |
| `cmd/account/create.go` | Pass ctx to RunE call sites |
| `cmd/account/create_actions.go` | Thread ctx |
| `cmd/account/edit.go` | Thread ctx |
| `cmd/account/edit_actions.go` | Thread ctx |
| `cmd/account/list.go` | Thread ctx |
| `cmd/account/delete.go` | Thread ctx |
| `cmd/transaction/show.go` | Thread ctx |
| `cmd/transaction/list.go` | Thread ctx |
| `cmd/transaction/clear.go` | Thread ctx |
| `cmd/transaction/delete.go` | Thread ctx |
| `cmd/transaction/edit.go` | Thread ctx |
| `cmd/transaction/edit_actions.go` | Thread ctx |
| `cmd/root.go` | `initSysAcc` / `migrateLegacySysAcc` use `context.Background()` |

**Not changed** (no DB calls, no ctx needed):
- `internal/service/account_validation.go` — pure string validation
- `internal/service/transaction_validation.go` line 12 `ValidateSplitsBalance` — pure math (only `ValidateTransactionEdit` changes)
- Service methods: `GetTransactionRule`, `IsEditable`, `GetAllowedAccounts`, `GetDisplayAccount`, `GetDisplayOffsetAccount`, `GetDisplayAmount`, `ValidateSplitsMatchType`, `ValidateAccountName`, `ValidateFullAccountName`, `ValidateCurrency`, `FormatAccountName`, `GetRootNameByType`

---

## Task 1: Update DBTX Interface and Internal Store DB Calls

> This task is compile-safe — it changes only internal store details, not any public method signatures.

**Files:**
- Modify: `internal/store/sqlite.go`
- Modify: `internal/store/sqlite_account.go`
- Modify: `internal/store/sqlite_transaction.go`
- Modify: `internal/store/sqlite_reconcile.go`

- [ ] **Step 1: Update `DBTX` interface in `internal/store/sqlite.go`**

Replace lines 19–24 (the `DBTX` interface):

```go
type DBTX interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	PrepareContext(ctx context.Context, query string) (*sql.Stmt, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}
```

Both `*sql.DB` and `*sql.Tx` already satisfy this interface, so `ExecTx` keeps working unchanged.

- [ ] **Step 2: Update internal DB calls in `internal/store/sqlite_account.go`**

The public method signatures do NOT change yet. Only the internal `s.db.Xxx` call sites change. Problem: the methods don't have a `ctx` parameter yet, so use `context.Background()` as a temporary placeholder to keep things compiling. (In Task 3, the real ctx will be threaded.)

Apply this pattern to every method:
- `s.db.Prepare(q)` → `s.db.PrepareContext(context.Background(), q)`
- After prepare: `stmt.QueryRow(args...)` → `stmt.QueryRowContext(context.Background(), args...)`
- After prepare: `stmt.Exec(args...)` → `stmt.ExecContext(context.Background(), args...)`
- `s.db.Query(q, args...)` → `s.db.QueryContext(context.Background(), q, args...)`
- `s.db.QueryRow(q, args...)` → `s.db.QueryRowContext(context.Background(), q, args...)`
- `s.db.Exec(q, args...)` → `s.db.ExecContext(context.Background(), q, args...)`

Add `"context"` to the import block.

Affected methods (13 total):
`CreateAccount`, `GetAllAccounts`, `GetAccountByName`, `GetAccountByID`, `AccountExists`, `GetAllAccountBalances`, `HasChildAccounts`, `GetAccountsByType`, `GetAccountBalance`, `AccountHasTransactions`, `RenameAccount`, `DeleteAccount`, `UpdateAccountMetadata`

- [ ] **Step 3: Update internal DB calls in `internal/store/sqlite_transaction.go`**

Apply the same `context.Background()` placeholder pattern. Add `"context"` to imports.

Affected methods (14 in this file):
`CreateTransactionWithSplits`, `GetTransactionByID`, `GetTransactionsByAccount`, `GetTransactionsByDateRange`, `GetAllTransactions`, `UpdateTransactionStatus`, `DeleteTransaction`, `UpdateTransactionBasic`, `UpdateSplit`, `DeleteSplit`, `CreateSplit`, `GetSplitsByTransaction`, `GetSplitsWithAccountsByDateRange`, `GetSplitsWithAccountsByTransaction`

(`GetUnreconciledTransactionsByAccount` lives in `sqlite_reconcile.go` — handled in Step 4.)

Exact call replacement example for `CreateTransactionWithSplits`:
```go
stmtTx, err := s.db.PrepareContext(context.Background(), `
    INSERT INTO transactions (timestamp, description, status, external_id, type)
    VALUES (?, ?, ?, ?, ?)
    RETURNING id;
`)
// ...
err = stmtTx.QueryRowContext(context.Background(), tx.Timestamp, tx.Description, tx.Status, tx.ExternalID, tx.Type).Scan(&newTxID)
// ...
stmtSplit, err := s.db.PrepareContext(context.Background(), `
    INSERT INTO splits (transaction_id, account_id, amount, currency, memo)
    VALUES (?, ?, ?, ?, ?);
`)
// ...
_, err := stmtSplit.ExecContext(context.Background(), newTxID, split.AccountID, split.Amount, split.Currency, split.Memo)
```

- [ ] **Step 4: Update internal DB calls in `internal/store/sqlite_reconcile.go`**

Apply the same `context.Background()` placeholder pattern. Add `"context"` to imports.

Affected methods (5 total):
`GetUnreconciledTransactionsByAccount`, `MarkSplitsReconciledByAccount`, `BulkUpdateTransactionStatus`, `GetLastReconciledBalance`, `SetLastReconciledBalance`

- [ ] **Step 5: Verify Task 1 compiles and tests pass**

```bash
go build ./...
go test ./...
```

Expected: all pass. If any method in store has a mismatched signature (e.g., used wrong method name), fix it now.

- [ ] **Step 6: Commit Task 1**

```bash
git add internal/store/sqlite.go internal/store/sqlite_account.go internal/store/sqlite_transaction.go internal/store/sqlite_reconcile.go
git commit -m "refactor: switch DBTX interface to *Context methods (internal only)"
```

---

## Task 2: Update Repository Interfaces

> After this task the build breaks. It will not pass until Task 10 is complete.

**Files:**
- Modify: `internal/repository/interfaces.go`

- [ ] **Step 1: Add `ctx context.Context` as first parameter to all interface methods**

Replace the entire file content:

```go
package repository

import (
	"context"

	"github.com/hance08/kea/internal/model"
)

type AccountRepository interface {
	CreateAccount(ctx context.Context, name string, accType model.AccountType, currency, description string, parentID *int64) (int64, error)
	GetAllAccounts(ctx context.Context) ([]*model.Account, error)
	GetAccountByName(ctx context.Context, name string) (*model.Account, error)
	GetAccountByID(ctx context.Context, id int64) (*model.Account, error)
	AccountExists(ctx context.Context, name string) (bool, error)
	GetAccountsByType(ctx context.Context, accType model.AccountType) ([]*model.Account, error)
	GetAccountBalance(ctx context.Context, accountID int64) (int64, error)
	GetAllAccountBalances(ctx context.Context, asOf int64) (map[int64]int64, error)
	HasChildAccounts(ctx context.Context, accountID int64) (bool, error)
	AccountHasTransactions(ctx context.Context, accountID int64) (bool, error)
	DeleteAccount(ctx context.Context, accountID int64) error
	RenameAccount(ctx context.Context, oldName, newName string) error
	UpdateAccountMetadata(ctx context.Context, accountID int64, description string, isHidden bool) error
}

type TransactionRepository interface {
	CreateTransactionWithSplits(ctx context.Context, tx model.Transaction, splits []model.Split) (int64, error)
	GetTransactionByID(ctx context.Context, txID int64) (*model.Transaction, error)
	GetTransactionsByAccount(ctx context.Context, accountID int64, limit int) ([]*model.Transaction, error)
	GetTransactionsByDateRange(ctx context.Context, startTime, endTime int64) ([]*model.Transaction, error)
	GetAllTransactions(ctx context.Context, limit int) ([]*model.Transaction, error)

	UpdateTransactionStatus(ctx context.Context, txID int64, status model.TransactionStatus) error
	DeleteTransaction(ctx context.Context, txID int64) error
	UpdateTransactionBasic(ctx context.Context, txID int64, description string, timestamp int64, status model.TransactionStatus, txType model.TransactionType) error

	CreateSplit(ctx context.Context, txID int64, split *model.Split) (int64, error)
	UpdateSplit(ctx context.Context, splitID int64, accountID int64, amount int64, currency string, memo string) error
	DeleteSplit(ctx context.Context, splitID int64) error
	GetSplitsByTransaction(ctx context.Context, txID int64) ([]*model.Split, error)

	GetSplitsWithAccountsByDateRange(ctx context.Context, startTime, endTime int64) (map[int64][]model.SplitDetail, error)
	GetSplitsWithAccountsByTransaction(ctx context.Context, txID int64) ([]model.SplitDetail, error)
	GetUnreconciledTransactionsByAccount(ctx context.Context, accountID int64) ([]*model.ReconcileEntry, error)
	BulkUpdateTransactionStatus(ctx context.Context, txIDs []int64, status model.TransactionStatus) error
	MarkSplitsReconciledByAccount(ctx context.Context, accountID int64, txIDs []int64) (int64, error)
	GetLastReconciledBalance(ctx context.Context, accountID int64) (int64, error)
	SetLastReconciledBalance(ctx context.Context, accountID int64, balance int64) error
}

type Repository interface {
	AccountRepository
	TransactionRepository
}

type TransactionManager interface {
	ExecTx(ctx context.Context, fn func(Repository) error) error
}
```

---

## Task 3: Update Store Account Method Signatures

**Files:**
- Modify: `internal/store/sqlite_account.go`

- [ ] **Step 1: Add `ctx context.Context` as first parameter to each Store method and replace placeholder `context.Background()` with `ctx`**

Apply this to all 13 methods. Example showing full before/after for `CreateAccount`:

Before:
```go
func (s *Store) CreateAccount(name string, accType model.AccountType, currency string, description string, parentID *int64) (int64, error) {
	stmt, err := s.db.PrepareContext(context.Background(), `...`)
	// ...
	err = stmt.QueryRowContext(context.Background(), name, ...).Scan(&newID)
```

After:
```go
func (s *Store) CreateAccount(ctx context.Context, name string, accType model.AccountType, currency string, description string, parentID *int64) (int64, error) {
	stmt, err := s.db.PrepareContext(ctx, `...`)
	// ...
	err = stmt.QueryRowContext(ctx, name, ...).Scan(&newID)
```

Apply the same pattern — add `ctx context.Context` as first param, replace all `context.Background()` with `ctx` — to every method:

- `GetAllAccounts(ctx context.Context) ([]*model.Account, error)`
- `GetAccountByName(ctx context.Context, name string) (*model.Account, error)`
- `GetAccountByID(ctx context.Context, id int64) (*model.Account, error)`
- `AccountExists(ctx context.Context, name string) (bool, error)`
- `GetAllAccountBalances(ctx context.Context, asOf int64) (map[int64]int64, error)`
- `HasChildAccounts(ctx context.Context, accountID int64) (bool, error)`
- `GetAccountsByType(ctx context.Context, accType model.AccountType) ([]*model.Account, error)`
- `GetAccountBalance(ctx context.Context, accountID int64) (int64, error)`
- `AccountHasTransactions(ctx context.Context, accountID int64) (bool, error)`
- `RenameAccount(ctx context.Context, oldName, newName string) error`
- `DeleteAccount(ctx context.Context, accountID int64) error`
- `UpdateAccountMetadata(ctx context.Context, accountID int64, description string, isHidden bool) error`

The `scanAccounts` private helper method does NOT change (it operates on `*sql.Rows`, no DB calls).

---

## Task 4: Update Store Transaction and Reconcile Method Signatures

**Files:**
- Modify: `internal/store/sqlite_transaction.go`
- Modify: `internal/store/sqlite_reconcile.go`

- [ ] **Step 1: Update `internal/store/sqlite_transaction.go`**

Add `ctx context.Context` as first param and replace all `context.Background()` with `ctx`:

- `CreateTransactionWithSplits(ctx context.Context, tx model.Transaction, splits []model.Split) (int64, error)`
- `GetTransactionByID(ctx context.Context, txID int64) (*model.Transaction, error)`
- `GetTransactionsByAccount(ctx context.Context, accountID int64, limit int) ([]*model.Transaction, error)`
- `GetTransactionsByDateRange(ctx context.Context, startTime, endTime int64) ([]*model.Transaction, error)`
- `GetAllTransactions(ctx context.Context, limit int) ([]*model.Transaction, error)`
- `UpdateTransactionStatus(ctx context.Context, txID int64, status model.TransactionStatus) error`
- `DeleteTransaction(ctx context.Context, txID int64) error`
- `UpdateTransactionBasic(ctx context.Context, txID int64, description string, timestamp int64, status model.TransactionStatus, txType model.TransactionType) error`
- `UpdateSplit(ctx context.Context, splitID int64, accountID int64, amount int64, currency string, memo string) error`
- `DeleteSplit(ctx context.Context, splitID int64) error`
- `CreateSplit(ctx context.Context, txID int64, split *model.Split) (int64, error)`
- `GetSplitsByTransaction(ctx context.Context, txID int64) ([]*model.Split, error)`
- `GetSplitsWithAccountsByDateRange(ctx context.Context, startTime, endTime int64) (map[int64][]model.SplitDetail, error)`
- `GetSplitsWithAccountsByTransaction(ctx context.Context, txID int64) ([]model.SplitDetail, error)`

The `scanTransactions` private helper does NOT change.

- [ ] **Step 2: Update `internal/store/sqlite_reconcile.go`**

Add `ctx context.Context` as first param and replace all `context.Background()` with `ctx`:

- `GetUnreconciledTransactionsByAccount(ctx context.Context, accountID int64) ([]*model.ReconcileEntry, error)`
- `MarkSplitsReconciledByAccount(ctx context.Context, accountID int64, txIDs []int64) (int64, error)`
- `BulkUpdateTransactionStatus(ctx context.Context, txIDs []int64, status model.TransactionStatus) error`
- `GetLastReconciledBalance(ctx context.Context, accountID int64) (int64, error)`
- `SetLastReconciledBalance(ctx context.Context, accountID int64, balance int64) error`

Note: `MarkSplitsReconciledByAccount` at line 104 calls `s.BulkUpdateTransactionStatus(txIDs, model.StatusReconciled)` — this internal call must also pass ctx: `s.BulkUpdateTransactionStatus(ctx, txIDs, model.StatusReconciled)`.

---

## Task 5: Update Service Test Mocks

**Files:**
- Modify: `internal/service/testhelper_test.go`

- [ ] **Step 1: Add `_ context.Context` as first parameter to every mock method on `mockAccountRepo`**

The mock ignores ctx (it operates on in-memory maps), so use blank identifier. Apply to all 13 methods:

```go
func (m *mockAccountRepo) CreateAccount(_ context.Context, name string, accType model.AccountType, currency, description string, parentID *int64) (int64, error) {
func (m *mockAccountRepo) GetAllAccounts(_ context.Context) ([]*model.Account, error) {
func (m *mockAccountRepo) GetAccountByName(_ context.Context, name string) (*model.Account, error) {
func (m *mockAccountRepo) GetAccountByID(_ context.Context, id int64) (*model.Account, error) {
func (m *mockAccountRepo) AccountExists(_ context.Context, name string) (bool, error) {
func (m *mockAccountRepo) GetAccountsByType(_ context.Context, accType model.AccountType) ([]*model.Account, error) {
func (m *mockAccountRepo) GetAccountBalance(_ context.Context, accountID int64) (int64, error) {
func (m *mockAccountRepo) GetAllAccountBalances(_ context.Context, asOf int64) (map[int64]int64, error) {
func (m *mockAccountRepo) HasChildAccounts(_ context.Context, accountID int64) (bool, error) {
func (m *mockAccountRepo) AccountHasTransactions(_ context.Context, accountID int64) (bool, error) {
func (m *mockAccountRepo) DeleteAccount(_ context.Context, accountID int64) error {
func (m *mockAccountRepo) RenameAccount(_ context.Context, oldName, newName string) error {
func (m *mockAccountRepo) UpdateAccountMetadata(_ context.Context, accountID int64, description string, isHidden bool) error {
```

- [ ] **Step 2: Add `_ context.Context` to every mock method on `mockTransactionRepo`**

Apply to all 14 methods:

```go
func (m *mockTransactionRepo) CreateTransactionWithSplits(_ context.Context, tx model.Transaction, splits []model.Split) (int64, error) {
func (m *mockTransactionRepo) GetTransactionByID(_ context.Context, txID int64) (*model.Transaction, error) {
func (m *mockTransactionRepo) GetTransactionsByAccount(_ context.Context, accountID int64, limit int) ([]*model.Transaction, error) {
func (m *mockTransactionRepo) GetTransactionsByDateRange(_ context.Context, startTime, endTime int64) ([]*model.Transaction, error) {
func (m *mockTransactionRepo) GetAllTransactions(_ context.Context, limit int) ([]*model.Transaction, error) {
func (m *mockTransactionRepo) UpdateTransactionStatus(_ context.Context, txID int64, status model.TransactionStatus) error {
func (m *mockTransactionRepo) DeleteTransaction(_ context.Context, txID int64) error {
func (m *mockTransactionRepo) UpdateTransactionBasic(_ context.Context, txID int64, description string, timestamp int64, status model.TransactionStatus, txType model.TransactionType) error {
func (m *mockTransactionRepo) CreateSplit(_ context.Context, txID int64, split *model.Split) (int64, error) {
func (m *mockTransactionRepo) UpdateSplit(_ context.Context, splitID int64, accountID int64, amount int64, currency string, memo string) error {
func (m *mockTransactionRepo) DeleteSplit(_ context.Context, splitID int64) error {
func (m *mockTransactionRepo) GetSplitsByTransaction(_ context.Context, txID int64) ([]*model.Split, error) {
func (m *mockTransactionRepo) GetSplitsWithAccountsByDateRange(_ context.Context, startTime, endTime int64) (map[int64][]model.SplitDetail, error) {
func (m *mockTransactionRepo) GetSplitsWithAccountsByTransaction(_ context.Context, txID int64) ([]model.SplitDetail, error) {
func (m *mockTransactionRepo) GetUnreconciledTransactionsByAccount(_ context.Context, accountID int64) ([]*model.ReconcileEntry, error) {
func (m *mockTransactionRepo) BulkUpdateTransactionStatus(_ context.Context, txIDs []int64, status model.TransactionStatus) error {
func (m *mockTransactionRepo) MarkSplitsReconciledByAccount(_ context.Context, accountID int64, txIDs []int64) (int64, error) {
func (m *mockTransactionRepo) GetLastReconciledBalance(_ context.Context, accountID int64) (int64, error) {
func (m *mockTransactionRepo) SetLastReconciledBalance(_ context.Context, accountID int64, balance int64) error {
```

---

## Task 6: Update Service Account Layer

**Files:**
- Modify: `internal/service/account_service.go`
- Modify: `internal/service/account_ops.go`

- [ ] **Step 1: Update `internal/service/account_service.go`**

Add `ctx context.Context` to every method that calls accRepo. Thread ctx to repo calls.

```go
func (as *AccountService) GetAllAccounts(ctx context.Context) ([]*model.Account, error) {
    return as.repo.GetAllAccounts(ctx)
}

func (as *AccountService) GetAccountByName(ctx context.Context, name string) (*model.Account, error) {
    acc, err := as.repo.GetAccountByName(ctx, name)
    // ... rest unchanged
}

func (as *AccountService) GetAccountsByType(ctx context.Context, accType model.AccountType) ([]*model.Account, error) {
    return as.repo.GetAccountsByType(ctx, accType)
}

func (as *AccountService) GetAccountBalance(ctx context.Context, accountID int64) (int64, error) {
    return as.repo.GetAccountBalance(ctx, accountID)
}

func (as *AccountService) GetAccountBalanceFormatted(ctx context.Context, accountID int64) (string, error) {
    balance, err := as.GetAccountBalance(ctx, accountID)
    // ... rest unchanged
}

func (as *AccountService) GetRootNameByType(accType string) (string, error) { // NO ctx — pure lookup table, no DB
    // ... unchanged
}

func (as *AccountService) CheckAccountExists(ctx context.Context, name string) (bool, error) {
    return as.repo.AccountExists(ctx, name)
}

func (as *AccountService) ValidateSelectableAccount(ctx context.Context, name string, allowedTypes []string) error {
    acc, err := as.GetAccountByName(ctx, name)
    // ... rest unchanged
}

func (as *AccountService) UpdateAccountMetadata(ctx context.Context, accountID int64, description string, isHidden bool) error {
    return as.repo.UpdateAccountMetadata(ctx, accountID, description, isHidden)
}
```

Add `"context"` to imports.

- [ ] **Step 2: Update `internal/service/account_ops.go`**

> **Scope note:** This PR threads `ctx` through the existing structure. It does NOT change atomicity boundaries. The known atomicity bug in `CreateAccountWithBalance` (issue #29 — `CreateAccount` and `createOpeningBalance` are not in the same transaction) stays unchanged here and will be fixed in a separate PR. Keep the existing two-step structure; just plumb `ctx` through it.

Add ctx to every method and thread it through. Replace `context.Background()` with `ctx`:

```go
func (as *AccountService) CreateAccount(ctx context.Context, name string, accType model.AccountType, currency, description string, parentID *int64) (*model.Account, error) {
    // ...validation unchanged...
    _, err := as.repo.CreateAccount(ctx, name, accType, currency, description, parentID)
    // ...
}

func (as *AccountService) CreateAccountWithBalance(ctx context.Context, name string, accType model.AccountType, currency, description string, parentID *int64, balance int64) (*model.Account, error) {
    account, err := as.CreateAccount(ctx, name, accType, currency, description, parentID)
    if err != nil {
        return nil, err
    }
    if balance != 0 {
        if err := as.createOpeningBalance(ctx, account, balance); err != nil {
            return account, fmt.Errorf("account created but failed to set opening balance: %w", err)
        }
    }
    return account, nil
}

func (as *AccountService) createOpeningBalance(ctx context.Context, account *model.Account, amountInCents int64) error {
    // ...existing setup unchanged (currency, equityAccountName, balanceAmount, equityAmount, tx)...
    return as.tm.ExecTx(ctx, func(repo repository.Repository) error {
        // existing closure body unchanged — keeps using the closure-bound `repo`,
        // just add ctx to every repo.X(...) call:
        equityAcc, err := repo.GetAccountByName(ctx, equityAccountName)
        if err != nil {
            if !errors.Is(err, store.ErrRecordNotFound) {
                return fmt.Errorf("failed to look up %q: %w", equityAccountName, err)
            }
            newID, createErr := repo.CreateAccount(ctx, equityAccountName, model.AccountTypeEquity, currency, "Opening Balances (System Account)", nil)
            if createErr != nil {
                return fmt.Errorf("failed to create %q: %w", equityAccountName, createErr)
            }
            equityAcc = &model.Account{ID: newID}
        }
        splits := []model.Split{
            {AccountID: account.ID, Amount: balanceAmount, Currency: currency, Memo: model.OpeningAccountMemo},
            {AccountID: equityAcc.ID, Amount: equityAmount, Currency: currency, Memo: model.OpeningAccountMemo},
        }
        _, err = repo.CreateTransactionWithSplits(ctx, tx, splits)
        return err
    })
}

func (as *AccountService) DeleteAccountByName(ctx context.Context, name string) error {
    acc, err := as.repo.GetAccountByName(ctx, name)
    // ...
    hasChildren, err := as.repo.HasChildAccounts(ctx, acc.ID)
    // ...
    hasTx, err := as.repo.AccountHasTransactions(ctx, acc.ID)
    // ...
    return as.repo.DeleteAccount(ctx, acc.ID)
}

func (as *AccountService) RenameAccount(ctx context.Context, oldName, newSegment string) error {
    oldAcc, err := as.repo.GetAccountByName(ctx, oldName)
    // ...
    _, err = as.repo.GetAccountByName(ctx, newFullName)
    // ...
    return as.repo.RenameAccount(ctx, oldName, newFullName)
}
```

`FormatAccountName(prefix, name string) string` — pure string method, NO ctx.

Note on `createOpeningBalance`: it currently opens its own `ExecTx` and uses the closure-bound `repo` (verified in the current code). Keep that structure; only swap `context.Background()` for the new `ctx` parameter and add `ctx` to every `repo.X(...)` call inside the closure. Do NOT lift the `ExecTx` up into `CreateAccountWithBalance` in this PR — that's the #29 fix and belongs in its own change.

---

## Task 7: Update Service Transaction Layer

**Files:**
- Modify: `internal/service/transaction_service.go`
- Modify: `internal/service/transaction_ops.go`
- Modify: `internal/service/transaction_validation.go`

- [ ] **Step 1: Update `internal/service/transaction_service.go`**

```go
func (ts *TransactionService) GetTransactionRule(txType model.TransactionType) (model.TransactionRule, error) { // NO ctx — pure table lookup
func (ts *TransactionService) GetTransactionByID(ctx context.Context, txID int64) (*model.TransactionDetail, error) {
    tx, err := ts.txRepo.GetTransactionByID(ctx, txID)
    // ...
    splits, err := ts.txRepo.GetSplitsWithAccountsByTransaction(ctx, txID)
    // ...
}
func (ts *TransactionService) GetRecentTransactions(ctx context.Context, limit int) ([]*model.Transaction, error) {
    return ts.txRepo.GetAllTransactions(ctx, limit)
}
func (ts *TransactionService) GetTransactionHistory(ctx context.Context, accountName string, limit int) ([]*model.Transaction, error) {
    acc, err := ts.accRepo.GetAccountByName(ctx, accountName)
    // ...
    return ts.txRepo.GetTransactionsByAccount(ctx, acc.ID, limit)
}
```

- [ ] **Step 2: Update `internal/service/transaction_ops.go`**

Replace all `context.Background()` occurrences with the incoming `ctx`. Key methods:

```go
func (ts *TransactionService) CreateTransaction(ctx context.Context, input model.TransactionDetail) (int64, error) {
    err := ts.tm.ExecTx(ctx, func(repo repository.Repository) error {
        txID, err := repo.CreateTransactionWithSplits(ctx, tx, splits)
        // ...
    })
}

func (ts *TransactionService) CreateSimpleTransaction(ctx context.Context, fromAccount, toAccount string, amount int64, desc string, timestamp int64, status model.TransactionStatus, txType model.TransactionType) (model.TransactionDetail, error) {
    fromAcc, err := ts.accRepo.GetAccountByName(ctx, fromAccount)
    toAcc, err := ts.accRepo.GetAccountByName(ctx, toAccount)
    // ...
    txID, err := ts.CreateTransaction(ctx, input)
    // ...
}

func (ts *TransactionService) CreateTransactionFromSplits(ctx context.Context, splits []model.SplitDetail, desc string, timestamp int64, status model.TransactionStatus, txType model.TransactionType) (model.TransactionDetail, error) {
    // ...
    txID, err := ts.CreateTransaction(ctx, input)
    // ...
}

func (ts *TransactionService) DeleteTransaction(ctx context.Context, txID int64) error {
    tx, err := ts.txRepo.GetTransactionByID(ctx, txID)
    // ...
    return ts.txRepo.DeleteTransaction(ctx, txID)
}

func (ts *TransactionService) UpdateTransactionStatus(ctx context.Context, txID int64, status model.TransactionStatus) error {
    tx, err := ts.txRepo.GetTransactionByID(ctx, txID)
    // ...
    return ts.txRepo.UpdateTransactionStatus(ctx, txID, status)
}

func (ts *TransactionService) UpdateTransactionComplete(ctx context.Context, txID int64, description string, timestamp int64, status model.TransactionStatus, txType model.TransactionType, splits []model.SplitDetail) error {
    return ts.tm.ExecTx(ctx, func(repo repository.Repository) error {
        // All repo calls pass ctx
        tx, err := repo.GetTransactionByID(ctx, txID)
        // ...
        err = repo.UpdateTransactionBasic(ctx, txID, ...)
        // ...
        for _, s := range toDelete {
            err := repo.DeleteSplit(ctx, s.ID)
        }
        // ...
    })
}
```

`IsEditable(detail *model.TransactionDetail) (bool, NotEditableReason)` — NO ctx (pure domain logic).

- [ ] **Step 3: Update `ValidateTransactionEdit` in `internal/service/transaction_validation.go`**

```go
func (ts *TransactionService) ValidateTransactionEdit(ctx context.Context, splits []model.SplitDetail) error {
    // ...
    for i, split := range splits {
        _, err := ts.accRepo.GetAccountByID(ctx, split.AccountID)
        // ...
    }
}
```

`ValidateSplitsBalance` — NO ctx (pure math).

---

## Task 8: Update Service Reconcile and Report Layers

**Files:**
- Modify: `internal/service/reconcile_ops.go`
- Modify: `internal/service/report_service.go`

- [ ] **Step 1: Update `internal/service/reconcile_ops.go`**

```go
func (ts *TransactionService) GetUnreconciledByAccount(ctx context.Context, accountID int64) ([]*model.ReconcileEntry, int64, error) {
    entries, err := ts.txRepo.GetUnreconciledTransactionsByAccount(ctx, accountID)
    // ...
    lastBalance, err := ts.txRepo.GetLastReconciledBalance(ctx, accountID)
    // ...
}

func (ts *TransactionService) ReconcileTransactions(ctx context.Context, accountID int64, statementBalance int64, txIDs []int64) (int64, error) {
    if err := ts.tm.ExecTx(ctx, func(repo repository.Repository) error {
        rowsAffected, err := repo.MarkSplitsReconciledByAccount(ctx, accountID, txIDs)
        // ...
        return repo.SetLastReconciledBalance(ctx, accountID, statementBalance)
    }); err != nil {
        // ...
    }
}
```

- [ ] **Step 2: Update `internal/service/report_service.go`**

```go
func (ts *TransactionService) GetNetWorthAt(ctx context.Context, endTime int64) (int64, error) {
    balances, err := ts.accRepo.GetAllAccountBalances(ctx, endTime)
    // ...
}

func (ts *TransactionService) buildReportMaps(ctx context.Context, startTime, endTime int64, includeIncome, includeExpense bool) (incomeByAccount, expenseByAccount map[string]*model.ReportRow, err error) {
    splitsMap, err := ts.txRepo.GetSplitsWithAccountsByDateRange(ctx, startTime, endTime)
    // ...
}

func (ts *TransactionService) GenerateIncomeStatement(ctx context.Context, startTime, endTime int64) (*model.ReportResult, error) {
    income, expense, err := ts.buildReportMaps(ctx, startTime, endTime, true, true)
    // ...
}

func (ts *TransactionService) GenerateIncomeBreakdown(ctx context.Context, startTime, endTime int64) (*model.ReportResult, error) {
    income, _, err := ts.buildReportMaps(ctx, startTime, endTime, true, false)
    // ...
}

func (ts *TransactionService) GenerateExpenseBreakdown(ctx context.Context, startTime, endTime int64) (*model.ReportResult, error) {
    _, expense, err := ts.buildReportMaps(ctx, startTime, endTime, false, true)
    // ...
}

func (ts *TransactionService) GenerateBalanceSheet(ctx context.Context, asOf int64) (*model.BalanceSheetResult, error) {
    balances, err := ts.accRepo.GetAllAccountBalances(ctx, asOf)
    // ...
    accounts, err := ts.accRepo.GetAllAccounts(ctx)
    // ...
}
```

Private helpers `offsetAccountName`, `getOrCreateRowWithOffset`, `rowsFromMap` — NO ctx (pure logic).

---

## Task 9: Update cmd Interfaces

> Every cmd-level interface that mirrors a DB-touching service method must add `ctx context.Context`.

**Files:**
- Modify: `cmd/add_types.go`
- Modify: `cmd/report_types.go`
- Modify: `cmd/reconcile.go`
- Modify: `cmd/transaction/list.go`
- Modify: `cmd/transaction/edit_types.go`
- Modify: `cmd/transaction/clear.go`
- Modify: `cmd/transaction/show.go`
- Modify: `cmd/transaction/delete.go`
- Modify: `cmd/account/list.go`
- Modify: `cmd/account/delete.go`
- Modify: `cmd/account/edit_types.go`
- Modify: `cmd/account/create_types.go`

- [ ] **Step 1: Update `cmd/add_types.go`**

```go
type AddProvider interface {
    GetAllAccounts(ctx context.Context) ([]*model.Account, error)
    GetAccountBalanceFormatted(ctx context.Context, accountID int64) (string, error)
    GetAccountByName(ctx context.Context, name string) (*model.Account, error)
    ValidateSelectableAccount(ctx context.Context, name string, allowedTypes []string) error
}

type TransactionProvider interface {
    GetTransactionRule(mode model.TransactionType) (model.TransactionRule, error)  // no ctx — pure lookup
    CreateSimpleTransaction(ctx context.Context, fromAccount string, toAccount string, amount int64, desc string, timestamp int64, status model.TransactionStatus, txType model.TransactionType) (model.TransactionDetail, error)
    CreateTransactionFromSplits(ctx context.Context, splits []model.SplitDetail, desc string, timestamp int64, status model.TransactionStatus, txType model.TransactionType) (model.TransactionDetail, error)
}
```

Add `"context"` to imports.

- [ ] **Step 2: Update `cmd/report_types.go`**

```go
type ReportProvider interface {
    GenerateIncomeStatement(ctx context.Context, startTime, endTime int64) (*model.ReportResult, error)
    GenerateIncomeBreakdown(ctx context.Context, startTime, endTime int64) (*model.ReportResult, error)
    GenerateExpenseBreakdown(ctx context.Context, startTime, endTime int64) (*model.ReportResult, error)
    GenerateBalanceSheet(ctx context.Context, asOf int64) (*model.BalanceSheetResult, error)
    GetNetWorthAt(ctx context.Context, endTime int64) (int64, error)
}
```

Add `"context"` to imports.

- [ ] **Step 3: Update `cmd/reconcile.go`**

```go
type reconcileAccountProvider interface {
    GetAccountByName(ctx context.Context, name string) (*model.Account, error)
}

type reconcileTxProvider interface {
    GetUnreconciledByAccount(ctx context.Context, accountID int64) ([]*model.ReconcileEntry, int64, error)
    ReconcileTransactions(ctx context.Context, accountID int64, statementBalance int64, txIDs []int64) (int64, error)
}
```

Add `"context"` to imports.

- [ ] **Step 4: Update `cmd/transaction/list.go`**

```go
type ListProvider interface {
    GetTransactionHistory(ctx context.Context, accountName string, limit int) ([]*model.Transaction, error)
    GetRecentTransactions(ctx context.Context, limit int) ([]*model.Transaction, error)
    GetTransactionByID(ctx context.Context, txID int64) (*model.TransactionDetail, error)
    GetDisplayAccount(splits []model.SplitDetail, txType string) (string, error)         // no ctx — pure logic
    GetDisplayOffsetAccount(splits []model.SplitDetail, txType string, primaryAccount string) (string, error)  // no ctx
    GetDisplayAmount(splits []model.SplitDetail) (int64, string)                         // no ctx
}
```

Add `"context"` to imports.

- [ ] **Step 5: Update `cmd/transaction/edit_types.go`**

```go
type EditProvider interface {
    GetTransactionByID(ctx context.Context, txID int64) (*model.TransactionDetail, error)
    IsEditable(detail *model.TransactionDetail) (bool, service.NotEditableReason)                       // no ctx
    GetAllowedAccounts(txType model.TransactionType, currentAccountType model.AccountType, allAccounts []*model.Account) []*model.Account // no ctx
    ValidateTransactionEdit(ctx context.Context, splits []model.SplitDetail) error
    ValidateSplitsMatchType(txType model.TransactionType, splits []model.SplitDetail) error             // no ctx
    UpdateTransactionComplete(ctx context.Context, txID int64, description string, timestamp int64, status model.TransactionStatus, txType model.TransactionType, splits []model.SplitDetail) error
}

type AccountProvider interface {
    GetAccountByName(ctx context.Context, name string) (*model.Account, error)
    GetAllAccounts(ctx context.Context) ([]*model.Account, error)
}
```

Add `"context"` to imports.

- [ ] **Step 6: Update `cmd/transaction/clear.go`**

```go
type ClearProvider interface {
    UpdateTransactionStatus(ctx context.Context, txID int64, status model.TransactionStatus) error
}
```

Add `"context"` to imports.

- [ ] **Step 7: Update `cmd/transaction/show.go`**

```go
type ShowProvider interface {
    GetTransactionByID(ctx context.Context, txID int64) (*model.TransactionDetail, error)
}
```

Add `"context"` to imports.

- [ ] **Step 8: Update `cmd/transaction/delete.go`**

```go
type TxDeleteProvider interface {
    GetTransactionByID(ctx context.Context, txID int64) (*model.TransactionDetail, error)
    DeleteTransaction(ctx context.Context, txID int64) error
}
```

Add `"context"` to imports.

- [ ] **Step 9: Update `cmd/account/list.go`**

```go
type AccountListProvider interface {
    GetAccountsByType(ctx context.Context, accType model.AccountType) ([]*model.Account, error)
    GetAllAccounts(ctx context.Context) ([]*model.Account, error)
    GetAccountBalance(ctx context.Context, id int64) (int64, error)
    GetAccountBalanceFormatted(ctx context.Context, id int64) (string, error)
}
```

Add `"context"` to imports.

- [ ] **Step 10: Update `cmd/account/delete.go`**

```go
type AccountDeleteProvider interface {
    GetAccountByName(ctx context.Context, name string) (*model.Account, error)
    DeleteAccountByName(ctx context.Context, name string) error
}
```

Add `"context"` to imports.

- [ ] **Step 11: Update `cmd/account/edit_types.go`**

```go
type EditProvider interface {
    GetAccountByName(ctx context.Context, name string) (*model.Account, error)
    GetAccountBalance(ctx context.Context, id int64) (int64, error)
    RenameAccount(ctx context.Context, oldName, newSegment string) error
    UpdateAccountMetadata(ctx context.Context, id int64, description string, isHidden bool) error
    ValidateAccountName(name string) error          // no ctx — pure validation
    CheckAccountExists(ctx context.Context, name string) (bool, error)
}
```

Add `"context"` to imports.

- [ ] **Step 12: Update `cmd/account/create_types.go`**

```go
type CreateProvider interface {
    GetAllAccounts(ctx context.Context) ([]*model.Account, error)
    GetAccountByName(ctx context.Context, name string) (*model.Account, error)
    GetRootNameByType(accType string) (string, error)       // no ctx — pure lookup
    CheckAccountExists(ctx context.Context, name string) (bool, error)
    FormatAccountName(prefix string, name string) string   // no ctx — pure string
    CreateAccountWithBalance(ctx context.Context, name string, accType model.AccountType, currency, description string, parentID *int64, balance int64) (*model.Account, error)
    ValidateAccountName(name string) error                 // no ctx — pure validation
    ValidateFullAccountName(name string) error             // no ctx — pure validation
    ValidateCurrency(currency string) error                // no ctx — pure validation
    GetAccountBalance(ctx context.Context, id int64) (int64, error)
}
```

Add `"context"` to imports.

---

## Task 10: Update cmd Call Sites and root.go

**Files:**
- Modify: `cmd/add.go`, `cmd/add_actions.go`, `cmd/add_test.go`
- Modify: `cmd/report.go`, `cmd/report_actions.go`
- Modify: `cmd/reconcile_actions.go`
- Modify: `cmd/root.go`
- Modify: `cmd/account/create.go`, `cmd/account/create_actions.go`
- Modify: `cmd/account/edit.go`, `cmd/account/edit_actions.go`
- Modify: `cmd/account/list.go`
- Modify: `cmd/account/delete.go`
- Modify: `cmd/transaction/show.go`
- Modify: `cmd/transaction/list.go`
- Modify: `cmd/transaction/clear.go`
- Modify: `cmd/transaction/delete.go`
- Modify: `cmd/transaction/edit.go`, `cmd/transaction/edit_actions.go`

The pattern for RunE-backed commands:
```go
RunE: func(cmd *cobra.Command, args []string) error {
    ctx := cmd.Context()
    return runner.run(ctx)
    // or: runner.Run(ctx, flags, cmd)
}
```

The pattern for runner methods: add `ctx context.Context` as first parameter and thread to every DB-touching call.

- [ ] **Step 1: Update `cmd/add.go` and `cmd/add_actions.go`**

`addRunner.Run` already receives `cmd *cobra.Command`. Extract ctx there:

```go
func (r *addRunner) Run(flags *addFlags, cmd *cobra.Command) error {
    ctx := cmd.Context()
    // ...
    input, err = r.runFromFlags(ctx, flags)
    // OR:
    input, err = r.runInteractive(ctx)
    // ...
    result, err = r.txSvc.CreateTransactionFromSplits(ctx, ...)
    result, err = r.txSvc.CreateSimpleTransaction(ctx, ...)
}

func (r *addRunner) runFromFlags(ctx context.Context, flags *addFlags) (addTransactionInput, error) { ... }
func (r *addRunner) runInteractive(ctx context.Context) (addTransactionInput, error) { ... }
func (r *addRunner) selectAccount(ctx context.Context, accounts []*model.Account, ...) (string, error) { ... }
func (r *addRunner) validateAccountSelectable(ctx context.Context, accountName string, allowedTypes []string, flagName string) error { ... }
```

Inside `runInteractive`: `accounts, err := r.accSvc.GetAllAccounts(ctx)` and `r.accSvc.GetAccountByName(ctx, ...)`.

`determineMode`, `parseDate`, `runFromSplitFlags` — check each method's body; only add ctx if they call DB methods.

- [ ] **Step 2: Update `cmd/add_test.go`**

Add `_ context.Context` to the mock:

```go
func (m *mockTransactionProvider) CreateSimpleTransaction(_ context.Context, fromAccount, toAccount string, ...) (model.TransactionDetail, error) {
func (m *mockTransactionProvider) CreateTransactionFromSplits(_ context.Context, splits []model.SplitDetail, ...) (model.TransactionDetail, error) {
```

`GetTransactionRule` — no ctx, no change.

Add `"context"` to imports if not already present.

- [ ] **Step 3: Update `cmd/report.go` and `cmd/report_actions.go`**

In `report.go` RunE:
```go
RunE: func(cmd *cobra.Command, args []string) error {
    runner := &reportRunner{...}
    return runner.run(cmd.Context())
},
```

In `report_actions.go`:
```go
func (r *reportRunner) run(ctx context.Context) error { ... }
func (r *reportRunner) runIncomeStatement(ctx context.Context) error {
    result, err := r.provider.GenerateIncomeStatement(ctx, start, end)
    currentNetWorth, err := r.provider.GetNetWorthAt(ctx, end)
    previousNetWorth, err := r.provider.GetNetWorthAt(ctx, prevEnd)
    // ...
}
func (r *reportRunner) runExpenseBreakdown(ctx context.Context) error {
    result, err := r.provider.GenerateExpenseBreakdown(ctx, start, end)
    // ...
}
func (r *reportRunner) runIncomeBreakdown(ctx context.Context) error {
    result, err := r.provider.GenerateIncomeBreakdown(ctx, start, end)
    // ...
}
func (r *reportRunner) runBalanceSheet(ctx context.Context) error {
    result, err := r.provider.GenerateBalanceSheet(ctx, end)
    // ...
}
// resolveDateRange — NO ctx (pure date math)
```

- [ ] **Step 4: Update `cmd/reconcile_actions.go`**

`reconcileRunner.Run` already receives `cmd *cobra.Command`:

```go
func (r *reconcileRunner) Run(cmd *cobra.Command, args []string) error {
    ctx := cmd.Context()
    acc, err := r.accSvc.GetAccountByName(ctx, accountName)
    // ...
    return r.runNonInteractive(ctx, acc)
    // or:
    return r.runInteractive(ctx, acc)
}

func (r *reconcileRunner) runNonInteractive(ctx context.Context, acc *model.Account) error {
    diff, err := r.txSvc.ReconcileTransactions(ctx, acc.ID, statementBalance, txIDs)
    // ...
}

func (r *reconcileRunner) runInteractive(ctx context.Context, acc *model.Account) error {
    entries, lastReconciledBalance, err := r.txSvc.GetUnreconciledByAccount(ctx, acc.ID)
    // ...
    diff, err := r.txSvc.ReconcileTransactions(ctx, acc.ID, statementBalance, selectedIDs)
    // ...
}
```

- [ ] **Step 5: Update `cmd/root.go`**

`initSysAcc` and `migrateLegacySysAcc` run before `rootCmd.Execute()`, so no Cobra context is available. Use `context.Background()`:

```go
func initSysAcc(svc *service.Service, cfg *config.Config) error {
    ctx := context.Background()
    if err := migrateLegacySysAcc(ctx, svc, cfg); err != nil { ... }
    _, err := svc.Account().GetAccountByName(ctx, targetName)
    // ...
    _, err = svc.Account().CreateAccount(ctx, targetName, ...)
}

func migrateLegacySysAcc(ctx context.Context, svc *service.Service, cfg *config.Config) error {
    _, err := svc.Account().GetAccountByName(ctx, legacyName)
    // ...
    _, err = svc.Account().GetAccountByName(ctx, targetName)
    // ...
    return svc.Account().RenameAccount(ctx, legacyName, targetName)
}
```

Add `"context"` to imports.

- [ ] **Step 6: Update `cmd/account/create.go` and `cmd/account/create_actions.go`**

In `create.go` RunE, extract `ctx := cmd.Context()` and pass to runner methods. In `create_actions.go`:

```go
func (r *createRunner) createAccount(ctx context.Context, input createInput) (*model.Account, error) {
    return r.accSvc.CreateAccountWithBalance(ctx, ...)
}
func (r *createRunner) buildFromParentName(ctx context.Context, parentName, currency string, input *createInput) error {
    parentAccount, err := r.accSvc.GetAccountByName(ctx, parentName)
    // ...
}
func (r *createRunner) buildFromTypeFlag(ctx context.Context, accType, currency string, input *createInput) error {
    allAccounts, err := r.accSvc.GetAllAccounts(ctx)
    // ...
}
```

Methods that only do string/logic work (`applyTypeSettings`, `applyParentSettings`) — check their bodies; add ctx only if they call DB methods.

- [ ] **Step 7: Update `cmd/account/edit.go` and `cmd/account/edit_actions.go`**

Extract `ctx := cmd.Context()` in RunE. In `edit_actions.go`:

```go
func (r *editRunner) runFromFlags(ctx context.Context, acc *model.Account, flags *editFlags, cmd *cobra.Command) (editInput, error) { ... }
func (r *editRunner) runInteractive(ctx context.Context, acc *model.Account) (editInput, error) {
    // Any call to r.svc.CheckAccountExists, r.svc.GetAccountByName etc. gets ctx
}
func (r *editRunner) applyChanges(ctx context.Context, acc *model.Account, input editInput) (string, error) {
    return r.svc.RenameAccount(ctx, ...) / r.svc.UpdateAccountMetadata(ctx, ...)
}
```

In `edit.go` RunE:
```go
ctx := cmd.Context()
acc, err := r.svc.GetAccountByName(ctx, accName)
updatedAcc, err := r.svc.GetAccountByName(ctx, finalName)
bal, err := r.svc.GetAccountBalance(ctx, updatedAcc.ID)
```

- [ ] **Step 8: Update `cmd/account/list.go`**

In RunE or runner method:
```go
ctx := cmd.Context()
accounts, err = r.svc.GetAccountsByType(ctx, model.AccountType(r.flags.Type))
accounts, err = r.svc.GetAllAccounts(ctx)
bal, err := r.svc.GetAccountBalance(ctx, acc.ID)
// GetAccountBalanceFormatted is passed as a callback — update to ctx-aware closure:
return views.NewAccountListView().Render(accounts, func(id int64) (string, error) {
    return r.svc.GetAccountBalanceFormatted(ctx, id)
})
```

Note: `GetAccountBalanceFormatted` is passed as a function value to `Render`. Since its signature now requires ctx, wrap it in a closure that captures ctx from the RunE scope.

- [ ] **Step 9: Update `cmd/account/delete.go`**

```go
ctx := cmd.Context()
acc, err := r.svc.GetAccountByName(ctx, name)
if err := r.svc.DeleteAccountByName(ctx, acc.Name); err != nil {
```

- [ ] **Step 10: Update `cmd/transaction/show.go`**

```go
ctx := cmd.Context()
detail, err := r.svc.GetTransactionByID(ctx, txID)
```

- [ ] **Step 11: Update `cmd/transaction/list.go`**

```go
ctx := cmd.Context()
return r.svc.GetTransactionHistory(ctx, r.flags.Account, r.flags.Limit)
return r.svc.GetRecentTransactions(ctx, r.flags.Limit)
detail, err := r.svc.GetTransactionByID(ctx, tx.ID)
```

`GetDisplayAccount`, `GetDisplayOffsetAccount`, `GetDisplayAmount` — no ctx, no change.

- [ ] **Step 12: Update `cmd/transaction/clear.go`**

```go
ctx := cmd.Context()
if err := r.svc.UpdateTransactionStatus(ctx, txID, model.StatusCleared); err != nil {
```

- [ ] **Step 13: Update `cmd/transaction/delete.go`**

```go
ctx := cmd.Context()
detail, err := r.svc.GetTransactionByID(ctx, txID)
if err := r.svc.DeleteTransaction(ctx, txID); err != nil {
```

- [ ] **Step 14: Update `cmd/transaction/edit.go` and `cmd/transaction/edit_actions.go`**

In `edit.go` RunE or runner entry point:
```go
ctx := cmd.Context()
detail, err := r.txSvc.GetTransactionByID(ctx, r.txID)
```

In `edit_actions.go`:
```go
func (r *editRunner) actionEditBasicInfo(ctx context.Context, detail *model.TransactionDetail) error { ... }
func (r *editRunner) actionQuickChangeAccount(ctx context.Context, detail *model.TransactionDetail) error {
    allAccounts, err := r.accSvc.GetAllAccounts(ctx)
    currentAcc, err := r.accSvc.GetAccountByName(ctx, targetSplit.AccountName)
    newAcc, _ := r.accSvc.GetAccountByName(ctx, newAccName)
}
func (r *editRunner) actionQuickChangeAmount(ctx context.Context, detail *model.TransactionDetail) error { ... }
func (r *editRunner) actionAddSplit(ctx context.Context, detail *model.TransactionDetail) error {
    accounts, err := r.accSvc.GetAllAccounts(ctx)
    acc, _ := r.accSvc.GetAccountByName(ctx, accName)
}
func (r *editRunner) actionEditSplit(ctx context.Context, detail *model.TransactionDetail) error {
    accounts, err := r.accSvc.GetAllAccounts(ctx)
    acc, _ := r.accSvc.GetAccountByName(ctx, newAccName)
}
func (r *editRunner) actionSave(ctx context.Context, detail *model.TransactionDetail) error {
    if err := r.txSvc.ValidateTransactionEdit(ctx, splits); err != nil {
    if err := r.txSvc.UpdateTransactionComplete(ctx, txID, ...); err != nil {
}
```

`actionDeleteSplit`, `actionEditType` — check their bodies; add ctx only if they call DB methods.
`determineMode` — no ctx (pure logic).

The main edit loop (likely in `edit.go` or a top-level runner method) passes ctx down to all action methods.

---

## Task 11: Verify and Commit

- [ ] **Step 1: Verify build**

```bash
go build ./...
```

Expected: exits 0 with no errors. If there are errors, they will point to remaining call sites that still use the old signature — fix each one.

- [ ] **Step 2: Run full test suite**

```bash
go test ./...
```

Expected: all tests pass. If any service-layer test fails with "too many arguments", check `testhelper_test.go` for any missed mock methods.

- [ ] **Step 3: Commit the full context-threading sweep**

```bash
git add -A
git commit -m "refactor: thread context.Context through all DB-touching methods

- DBTX interface uses *Context variants (ExecContext, QueryContext, etc.)
- All AccountRepository and TransactionRepository methods accept ctx
- Store implementations pass ctx to database/sql *Context calls
- Service methods accept and propagate ctx; ExecTx calls no longer use context.Background()
- cmd layer passes cmd.Context() from RunE; initSysAcc uses context.Background()
- Enables per-request cancellation, timeouts, and graceful shutdown

Closes #37"
```

---

## Self-Review Checklist

- [x] **Spec coverage**: All 4 steps from the issue are covered: DBTX (Task 1), AccountRepository (Task 2–3, 6), TransactionRepository (Task 2, 4, 7–8), service methods (Tasks 6–9), call sites (Task 10).
- [x] **No placeholders**: Every task shows exact method signatures. Task 10 step 14 ("check their bodies") is an instruction to read before writing — the signatures shown are complete for the methods that definitely change.
- [x] **Type consistency**: `ctx context.Context` is the first parameter everywhere. `_ context.Context` in mocks. ExecTx fn captures ctx from outer method — no changes needed to TransactionManager interface.
- [x] **Field names verified**: `AccountService` field is `repo` (not `accRepo`); `TransactionService` fields are `txRepo` and `accRepo`. Plan code blocks reflect this.
- [x] **Scope is bounded**: Pure mechanical ctx threading. Atomicity bug in `CreateAccountWithBalance` (issue #29) is explicitly NOT fixed here — `createOpeningBalance` keeps its own `ExecTx`. #29 is a separate PR.
- [x] **root.go edge case**: `initSysAcc`/`migrateLegacySysAcc` run pre-Execute; explicitly documented to use `context.Background()`.
- [x] **GetAccountBalanceFormatted as callback**: Documented in Task 10 Step 8 that it must be wrapped in a ctx-capturing closure when passed to views.
- [x] **Private store helpers**: `scanAccounts`, `scanTransactions` explicitly noted as NOT changing.
- [x] **Non-DB service methods**: Explicitly documented which methods skip ctx across all tasks.
