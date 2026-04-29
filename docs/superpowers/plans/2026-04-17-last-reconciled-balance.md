# Last Reconciled Balance Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Track a per-account "last reconciled balance" so users can enter the bank statement's actual closing balance instead of a net-change delta on every reconcile.

**Architecture:** A new `account_reconcile_state` table stores the running reconciled balance per account. The service fetches it when loading unreconciled entries (for TUI display) and updates it after every successful reconcile. The new difference formula is `statementBalance − (lastReconciledBalance + clearedBalance)`; the persisted value after each reconcile is `lastReconciledBalance + clearedBalance`.

**Tech Stack:** Go, SQLite (golang-migrate), charmbracelet/bubbletea, cobra, pterm.

---

## File Map

| Action | File | What changes |
|--------|------|-------------|
| Create | `migrations/0004_add_account_reconcile_state.up.sql` | New table |
| Modify | `internal/repository/interfaces.go` | Add 2 methods to `TransactionRepository` |
| Modify | `internal/store/sqlite_reconcile.go` | Implement those 2 methods |
| Modify | `internal/service/testhelper_test.go` | Extend mock with the 2 new methods + state |
| Modify | `internal/service/reconcile_ops_test.go` | Update signatures; add 4 new tests |
| Modify | `internal/service/reconcile_ops.go` | New diff formula + persist new balance |
| Modify | `cmd/reconcile.go` | Update `reconcileTxProvider` interface |
| Modify | `cmd/reconcile_actions.go` | Pass `lastReconciledBalance` to TUI |
| Modify | `ui/reconcile/model.go` | New field, new diff formula, display line |

---

## Task 1: Migration — add `account_reconcile_state` table

**Files:**
- Create: `migrations/0004_add_account_reconcile_state.up.sql`

- [ ] **Step 1: Create the migration file**

```sql
-- Per-account reconciliation state. Stores the running reconciled balance so
-- that the reconcile UI can use the bank statement's actual closing balance
-- instead of asking for a net-change delta on every session.
CREATE TABLE IF NOT EXISTS account_reconcile_state (
    account_id              INTEGER PRIMARY KEY,
    last_reconciled_balance INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE CASCADE
);
```

- [ ] **Step 2: Verify the build still compiles (migrations are embedded automatically)**

Run: `make build`
Expected: exits 0 with no errors.

- [ ] **Step 3: Commit**

```bash
git add migrations/0004_add_account_reconcile_state.up.sql
git commit -m "feat(migrations): add account_reconcile_state table for last reconciled balance"
```

---

## Task 2: Repository interface — add `GetLastReconciledBalance` and `SetLastReconciledBalance`

**Files:**
- Modify: `internal/repository/interfaces.go`

- [ ] **Step 1: Add two methods to `TransactionRepository`**

In `internal/repository/interfaces.go`, append inside the `TransactionRepository` interface (after the existing `MarkSplitsReconciledByAccount` entry):

```go
	// GetLastReconciledBalance returns the running reconciled balance for
	// accountID as persisted after the most recent successful reconcile.
	// Returns 0 if the account has never been reconciled.
	GetLastReconciledBalance(accountID int64) (int64, error)

	// SetLastReconciledBalance persists the new running reconciled balance for
	// accountID. Uses an upsert so the first call creates the row.
	SetLastReconciledBalance(accountID int64, balance int64) error
```

- [ ] **Step 2: Verify the build fails with "does not implement" errors (expected — store not updated yet)**

Run: `make build`
Expected: compile error on `*store.Store` not implementing `TransactionRepository`.

- [ ] **Step 3: Commit the interface change**

```bash
git add internal/repository/interfaces.go
git commit -m "feat(repository): add GetLastReconciledBalance and SetLastReconciledBalance to TransactionRepository"
```

---

## Task 3: Store implementation — `GetLastReconciledBalance` and `SetLastReconciledBalance`

**Files:**
- Modify: `internal/store/sqlite_reconcile.go`

- [ ] **Step 1: Add the `database/sql` import** (if not already present)

Open `internal/store/sqlite_reconcile.go`. The file currently imports `"fmt"` and `"strings"`. Add `"database/sql"` to the import block:

```go
import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/hance08/kea/internal/model"
)
```

- [ ] **Step 2: Append both methods at the end of `sqlite_reconcile.go`**

```go
// GetLastReconciledBalance returns the running reconciled balance for
// accountID. Returns 0 if the account has never been reconciled (no row in
// account_reconcile_state for this account yet).
func (s *Store) GetLastReconciledBalance(accountID int64) (int64, error) {
	var balance int64
	err := s.db.QueryRow(
		"SELECT last_reconciled_balance FROM account_reconcile_state WHERE account_id = ?",
		accountID,
	).Scan(&balance)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("failed to get last reconciled balance: %w", err)
	}
	return balance, nil
}

// SetLastReconciledBalance persists the new running reconciled balance for
// accountID. An upsert is used so that the first reconcile for an account
// inserts a row; subsequent reconciles update it.
func (s *Store) SetLastReconciledBalance(accountID int64, balance int64) error {
	_, err := s.db.Exec(`
        INSERT INTO account_reconcile_state (account_id, last_reconciled_balance)
        VALUES (?, ?)
        ON CONFLICT(account_id) DO UPDATE SET last_reconciled_balance = excluded.last_reconciled_balance
    `, accountID, balance)
	if err != nil {
		return fmt.Errorf("failed to set last reconciled balance: %w", err)
	}
	return nil
}
```

- [ ] **Step 3: Verify build now succeeds**

Run: `make build`
Expected: exits 0.

- [ ] **Step 4: Run all tests to confirm nothing broke**

Run: `go test ./...`
Expected: all PASS (existing tests rely on mocks, not the real store).

- [ ] **Step 5: Commit**

```bash
git add internal/store/sqlite_reconcile.go
git commit -m "feat(store): implement GetLastReconciledBalance and SetLastReconciledBalance"
```

---

## Task 4: Update mock — extend `mockTransactionRepo` with the two new methods

**Files:**
- Modify: `internal/service/testhelper_test.go`

- [ ] **Step 1: Add fields to `mockTransactionRepo`**

Locate the `mockTransactionRepo` struct. After the `markSplitsReconciledCalls` field block, add:

```go
	// last reconciled balance state
	lastReconciledBalances    map[int64]int64
	getLastReconciledBalErr   map[int64]error
	setLastReconciledBalCalls []struct {
		accountID int64
		balance   int64
	}
```

- [ ] **Step 2: Initialise the new fields in `newMockTransactionRepo`**

Inside the `return &mockTransactionRepo{...}` literal, add:

```go
		lastReconciledBalances:  make(map[int64]int64),
		getLastReconciledBalErr: make(map[int64]error),
```

- [ ] **Step 3: Add the two interface method implementations at the end of the mock**

Append after `MarkSplitsReconciledByAccount`:

```go
func (m *mockTransactionRepo) GetLastReconciledBalance(accountID int64) (int64, error) {
	if err, ok := m.getLastReconciledBalErr[accountID]; ok {
		return 0, err
	}
	return m.lastReconciledBalances[accountID], nil
}

func (m *mockTransactionRepo) SetLastReconciledBalance(accountID int64, balance int64) error {
	m.lastReconciledBalances[accountID] = balance
	m.setLastReconciledBalCalls = append(m.setLastReconciledBalCalls, struct {
		accountID int64
		balance   int64
	}{accountID, balance})
	return nil
}
```

- [ ] **Step 4: Verify build**

Run: `make build`
Expected: exits 0.

- [ ] **Step 5: Commit**

```bash
git add internal/service/testhelper_test.go
git commit -m "test(service): extend mockTransactionRepo with GetLastReconciledBalance and SetLastReconciledBalance"
```

---

## Task 5: Service tests — update signatures and add new tests

**Files:**
- Modify: `internal/service/reconcile_ops_test.go`

The service's `GetUnreconciledByAccount` will change from `([]*model.ReconcileEntry, error)` to `([]*model.ReconcileEntry, int64, error)`, and `ReconcileTransactions` will use the new formula. Update the test file entirely to reflect this and add new coverage.

- [ ] **Step 1: Write the failing tests (replace the entire file)**

```go
package service

import (
	"testing"

	"github.com/hance08/kea/internal/model"
)

// seedUnreconciled injects unreconciled entries into the mock for accountID.
func seedUnreconciled(txRepo *mockTransactionRepo, accountID int64, entries []*model.ReconcileEntry) {
	txRepo.unreconciledByAccount[accountID] = entries
}

// ── GetUnreconciledByAccount ──────────────────────────────────────────────────

func TestGetUnreconciledByAccount_ReturnsEntriesAndBalance(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Checking", Type: model.AccountTypeAsset})
	seedUnreconciled(txRepo, 1, []*model.ReconcileEntry{
		{ID: 10, Timestamp: 1000, Description: "Salary", Status: model.StatusCleared, Amount: 100000},
	})
	txRepo.lastReconciledBalances[1] = 50000

	entries, lastBal, err := svc.GetUnreconciledByAccount(1)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(entries))
	}
	if lastBal != 50000 {
		t.Errorf("expected lastReconciledBalance 50000, got %d", lastBal)
	}
}

func TestGetUnreconciledByAccount_FirstTime_ReturnsZeroBalance(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Checking", Type: model.AccountTypeAsset})
	// no entry in lastReconciledBalances → mock returns 0

	_, lastBal, err := svc.GetUnreconciledByAccount(1)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lastBal != 0 {
		t.Errorf("expected 0 for first reconcile, got %d", lastBal)
	}
}

// ── ReconcileTransactions ─────────────────────────────────────────────────────

func TestReconcileTransactions_FirstReconcile_ZeroDifference(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Checking", Type: model.AccountTypeAsset})
	seedUnreconciled(txRepo, 1, []*model.ReconcileEntry{
		{ID: 10, Amount: 100000},
		{ID: 11, Amount: -50000},
	})
	// No prior balance (first reconcile) → lastReconciledBalance = 0.
	// Cleared = 100000 + (-50000) = 50000; statementBalance = 50000 → diff = 0.

	diff, err := svc.ReconcileTransactions(1, 50000, []int64{10, 11})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diff != 0 {
		t.Errorf("expected diff 0, got %d", diff)
	}
	// New last reconciled balance should be 0 + 50000 = 50000.
	if txRepo.lastReconciledBalances[1] != 50000 {
		t.Errorf("expected persisted balance 50000, got %d", txRepo.lastReconciledBalances[1])
	}
	if len(txRepo.setLastReconciledBalCalls) != 1 {
		t.Errorf("expected 1 SetLastReconciledBalance call, got %d", len(txRepo.setLastReconciledBalCalls))
	}
}

func TestReconcileTransactions_SubsequentReconcile_CarriedBalance(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Checking", Type: model.AccountTypeAsset})
	seedUnreconciled(txRepo, 1, []*model.ReconcileEntry{
		{ID: 20, Amount: -15000}, // expense of $150
	})
	// lastReconciledBalance = 20000 ($200 from prior session).
	txRepo.lastReconciledBalances[1] = 20000
	// Bank now shows $50: diff = 5000 − (20000 + (−15000)) = 5000 − 5000 = 0.

	diff, err := svc.ReconcileTransactions(1, 5000, []int64{20})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diff != 0 {
		t.Errorf("expected diff 0, got %d", diff)
	}
	// New last reconciled balance = 20000 + (−15000) = 5000.
	if txRepo.lastReconciledBalances[1] != 5000 {
		t.Errorf("expected persisted balance 5000, got %d", txRepo.lastReconciledBalances[1])
	}
}

func TestReconcileTransactions_ForcedReconcile_PersistsActualCleared(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Checking", Type: model.AccountTypeAsset})
	seedUnreconciled(txRepo, 1, []*model.ReconcileEntry{
		{ID: 30, Amount: 20000}, // $200 transactions
	})
	// No prior balance. User enters $150 (bank shows $150) but cleared $200 — forced.
	// diff = 15000 − (0 + 20000) = −5000 (OVER $50).
	// Regardless of force, the service always reconciles and persists lastBal+cleared = 20000.

	diff, err := svc.ReconcileTransactions(1, 15000, []int64{30})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diff != -5000 {
		t.Errorf("expected diff -5000, got %d", diff)
	}
	// Persisted balance should be 0 + 20000 = 20000 (actual cleared, not statementBalance).
	if txRepo.lastReconciledBalances[1] != 20000 {
		t.Errorf("expected persisted balance 20000, got %d", txRepo.lastReconciledBalances[1])
	}
}

func TestReconcileTransactions_NonZeroDifference(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Checking", Type: model.AccountTypeAsset})
	seedUnreconciled(txRepo, 1, []*model.ReconcileEntry{
		{ID: 10, Amount: 100000},
	})
	// lastBal = 0; cleared = 100000; statementBalance = 120000 → diff = 20000 (SHORT).

	diff, err := svc.ReconcileTransactions(1, 120000, []int64{10})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diff != 20000 {
		t.Errorf("expected diff 20000, got %d", diff)
	}
	if len(txRepo.markSplitsReconciledCalls) != 1 {
		t.Fatalf("expected MarkSplitsReconciledByAccount to be called")
	}
}

func TestReconcileTransactions_UnknownTxID(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Checking", Type: model.AccountTypeAsset})
	seedUnreconciled(txRepo, 1, []*model.ReconcileEntry{
		{ID: 10, Amount: 100000},
	})

	_, err := svc.ReconcileTransactions(1, 100000, []int64{10, 99})

	if err == nil {
		t.Fatal("expected error for unknown transaction ID, got nil")
	}
	if len(txRepo.markSplitsReconciledCalls) != 0 {
		t.Error("MarkSplitsReconciledByAccount must not be called when validation fails")
	}
	if len(txRepo.setLastReconciledBalCalls) != 0 {
		t.Error("SetLastReconciledBalance must not be called when validation fails")
	}
}

func TestReconcileTransactions_AlreadyReconciledID(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Checking", Type: model.AccountTypeAsset})
	seedUnreconciled(txRepo, 1, []*model.ReconcileEntry{
		{ID: 10, Amount: 100000},
	})

	_, err := svc.ReconcileTransactions(1, 100000, []int64{10, 20})

	if err == nil {
		t.Fatal("expected error for already-reconciled ID, got nil")
	}
	if len(txRepo.setLastReconciledBalCalls) != 0 {
		t.Error("SetLastReconciledBalance must not be called when validation fails")
	}
}

func TestReconcileTransactions_EmptyIDs(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Checking", Type: model.AccountTypeAsset})

	_, err := svc.ReconcileTransactions(1, 100000, []int64{})

	if err == nil {
		t.Fatal("expected error for empty IDs, got nil")
	}
}

func TestReconcileTransactions_AccountNotFound(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	_, err := svc.ReconcileTransactions(99, 100000, []int64{10})

	if err == nil {
		t.Fatal("expected error for unknown account, got nil")
	}
	if len(txRepo.setLastReconciledBalCalls) != 0 {
		t.Error("SetLastReconciledBalance must not be called when account lookup fails")
	}
}
```

- [ ] **Step 2: Run the tests and confirm they fail (service not updated yet)**

Run: `go test ./internal/service/ -run TestReconcile -v`
Expected: compile errors (`GetUnreconciledByAccount` returns 2 values but test expects 3, etc.).

- [ ] **Step 3: Commit the failing tests**

```bash
git add internal/service/reconcile_ops_test.go
git commit -m "test(service): update reconcile tests for last-reconciled-balance feature"
```

---

## Task 6: Service implementation — new formula and balance persistence

**Files:**
- Modify: `internal/service/reconcile_ops.go`

- [ ] **Step 1: Replace the entire file**

```go
package service

import (
	"fmt"

	"github.com/hance08/kea/internal/model"
)

// GetUnreconciledByAccount returns all Pending/Cleared transactions that have
// a split touching the given account, together with the account's last
// reconciled balance. Used to populate the reconciliation TUI.
//
// The last reconciled balance is 0 if the account has never been reconciled.
func (ts *TransactionService) GetUnreconciledByAccount(accountID int64) ([]*model.ReconcileEntry, int64, error) {
	entries, err := ts.txRepo.GetUnreconciledTransactionsByAccount(accountID)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to load unreconciled transactions: %w", err)
	}
	lastBalance, err := ts.txRepo.GetLastReconciledBalance(accountID)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to load last reconciled balance: %w", err)
	}
	return entries, lastBalance, nil
}

// ReconcileTransactions marks the given transaction IDs as reconciled for
// accountID. It validates that every requested ID is in the unreconciled set
// for that account before writing anything.
//
// The returned difference is:
//
//	statementBalance − (lastReconciledBalance + clearedBalance)
//
// where clearedBalance is the sum of splits the caller selected in this
// session. A zero difference means the selected transactions exactly bridge
// the gap between the last reconciled balance and the new bank statement.
//
// The new running reconciled balance (lastReconciledBalance + clearedBalance)
// is always persisted after a successful reconcile — regardless of whether the
// difference is zero. The caller decides whether to warn on a non-zero diff.
func (ts *TransactionService) ReconcileTransactions(accountID int64, statementBalance int64, txIDs []int64) (int64, error) {
	if len(txIDs) == 0 {
		return 0, fmt.Errorf("no transactions selected for reconciliation")
	}

	// 1. Verify account exists.
	if _, err := ts.accRepo.GetAccountByID(accountID); err != nil {
		return 0, fmt.Errorf("account not found: %w", err)
	}

	// 2. Fetch last reconciled balance.
	lastBalance, err := ts.txRepo.GetLastReconciledBalance(accountID)
	if err != nil {
		return 0, fmt.Errorf("failed to load last reconciled balance: %w", err)
	}

	// 3. Fetch unreconciled transactions and build a valid-ID → amount map.
	entries, err := ts.txRepo.GetUnreconciledTransactionsByAccount(accountID)
	if err != nil {
		return 0, fmt.Errorf("failed to load unreconciled transactions: %w", err)
	}

	validAmounts := make(map[int64]int64, len(entries))
	for _, e := range entries {
		validAmounts[e.ID] = e.Amount
	}

	// 4. Validate every requested ID and accumulate the cleared balance.
	var clearedBalance int64
	for _, id := range txIDs {
		amount, ok := validAmounts[id]
		if !ok {
			return 0, fmt.Errorf("transaction ID %d is not in the unreconciled set for this account", id)
		}
		clearedBalance += amount
	}

	// 5. Mark the account's splits as reconciled (split-level tracking so that
	// multi-account transactions remain visible for other accounts).
	if err := ts.txRepo.MarkSplitsReconciledByAccount(accountID, txIDs); err != nil {
		return 0, fmt.Errorf("failed to reconcile transactions: %w", err)
	}

	// 6. Persist the new running reconciled balance.
	newBalance := lastBalance + clearedBalance
	if err := ts.txRepo.SetLastReconciledBalance(accountID, newBalance); err != nil {
		return 0, fmt.Errorf("failed to persist reconciled balance: %w", err)
	}

	return statementBalance - newBalance, nil
}
```

- [ ] **Step 2: Run the service tests and confirm they pass**

Run: `go test ./internal/service/ -v -run TestReconcile`
Expected: all PASS.

- [ ] **Step 3: Run full test suite**

Run: `go test ./...`
Expected: compile errors only in `cmd/` and `ui/reconcile/` packages (callers of the changed interface). Service tests all PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/service/reconcile_ops.go
git commit -m "feat(service): use last-reconciled-balance in diff formula and persist after reconcile"
```

---

## Task 7: Update cmd layer — interface and action wiring

**Files:**
- Modify: `cmd/reconcile.go`
- Modify: `cmd/reconcile_actions.go`

- [ ] **Step 1: Update `reconcileTxProvider` in `cmd/reconcile.go`**

Replace the interface definition:

```go
// reconcileTxProvider is the subset of TransactionService used by the runner.
type reconcileTxProvider interface {
	GetUnreconciledByAccount(accountID int64) ([]*model.ReconcileEntry, int64, error)
	ReconcileTransactions(accountID int64, statementBalance int64, txIDs []int64) (int64, error)
}
```

- [ ] **Step 2: Update `runInteractive` in `cmd/reconcile_actions.go`**

Replace the `runInteractive` function:

```go
func (r *reconcileRunner) runInteractive(acc *model.Account) error {
	// 1. Prompt for statement balance.
	balanceStr, err := prompts.PromptAmount(
		"Statement ending balance:",
		"Enter the closing balance from your bank statement (e.g. 2450.00)",
		prompts.ValidateAmountFormat(false),
	)
	if err != nil {
		return fmt.Errorf("prompt cancelled: %w", err)
	}

	statementBalance, err := utils.ParseAmount(balanceStr)
	if err != nil {
		return fmt.Errorf("invalid balance: %w", err)
	}

	// 2. Load unreconciled entries and the last reconciled balance.
	entries, lastReconciledBalance, err := r.txSvc.GetUnreconciledByAccount(acc.ID)
	if err != nil {
		return err
	}

	// 3. Run bubbletea TUI.
	m := reconcileui.NewModel(acc.Name, statementBalance, lastReconciledBalance, entries)
	prog := tea.NewProgram(m, tea.WithAltScreen())
	finalRaw, err := prog.Run()
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}
	finalModel, ok := finalRaw.(reconcileui.Model)
	if !ok {
		return fmt.Errorf("unexpected TUI model type: %T", finalRaw)
	}

	if finalModel.Cancelled() {
		pterm.Info.Println("Reconciliation cancelled.")
		return nil
	}

	selectedIDs := finalModel.SelectedIDs()
	if len(selectedIDs) == 0 {
		pterm.Info.Println("No transactions selected — nothing reconciled.")
		return nil
	}

	// 4. Persist.
	diff, err := r.txSvc.ReconcileTransactions(acc.ID, statementBalance, selectedIDs)
	if err != nil {
		return err
	}

	if r.flags.JSON {
		return writeReconcileJSON(acc.Name, len(selectedIDs), diff)
	}

	if diff == 0 {
		pterm.Success.Printf("Reconciled %d transaction(s) on %q — balanced!\n", len(selectedIDs), acc.Name)
	} else {
		pterm.Warning.Printf(
			"Reconciled %d transaction(s) on %q with a remaining difference of $%s.\n",
			len(selectedIDs), acc.Name, utils.FormatAmount(abs64(diff)),
		)
	}
	return nil
}
```

(The `runNonInteractive` function needs no changes: it calls `ReconcileTransactions` directly; the service now fetches `lastReconciledBalance` internally and returns the correct difference.)

- [ ] **Step 3: Verify build**

Run: `make build`
Expected: compile errors only in `ui/reconcile/` (TUI `NewModel` signature not yet updated).

- [ ] **Step 4: Commit**

```bash
git add cmd/reconcile.go cmd/reconcile_actions.go
git commit -m "feat(cmd): wire last-reconciled-balance through to reconcile TUI"
```

---

## Task 8: Update TUI model — new field, diff formula, and display line

**Files:**
- Modify: `ui/reconcile/model.go`

- [ ] **Step 1: Update `overheadLines`, add `lastReconciledBalance` field, update `NewModel`, `difference()`, and `View()`**

Apply the following changes to `ui/reconcile/model.go`:

**a) Change `overheadLines` from 8 to 9** (the new "LAST RECONCILED" line adds one overhead row):

```go
// overheadLines is the number of non-item lines in the view:
// account+badge (1) · blank (1) · statement (1) · last-reconciled (1) ·
// top-sep (1) · bottom-sep (1) · cleared (1) · blank (1) · hint (1) = 9
const overheadLines = 9
```

**b) Add `lastReconciledBalance` to `Model`** (after `statementBalance`):

```go
type Model struct {
	accountName           string
	statementBalance      int64
	lastReconciledBalance int64
	items                 []listItem
	cursor                int
	viewportOffset        int
	height                int
	confirmPending        bool
	cancelled             bool
	done                  bool
	keys                  keyMap
}
```

**c) Update `NewModel` to accept and store the new parameter**:

```go
// NewModel constructs the initial reconciliation model.
func NewModel(accountName string, statementBalance int64, lastReconciledBalance int64, entries []*model.ReconcileEntry) Model {
	items := make([]listItem, len(entries))
	for i, e := range entries {
		items[i] = listItem{entry: e}
	}
	return Model{
		accountName:           accountName,
		statementBalance:      statementBalance,
		lastReconciledBalance: lastReconciledBalance,
		items:                 items,
		keys:                  defaultKeyMap(),
	}
}
```

**d) Update `difference()` to use the new formula**:

```go
func (m Model) difference() int64 {
	return m.statementBalance - (m.lastReconciledBalance + m.clearedBalance())
}
```

**e) In `View()`, add the "LAST RECONCILED" line** immediately after the STATEMENT line (around line 215 in the original):

Replace:
```go
	stmtStr := utils.FormatAmount(m.statementBalance)
	sb.WriteString(fmt.Sprintf("STATEMENT: $%s · %d UNRECONCILED\n", stmtStr, len(m.items)))
```

With:
```go
	stmtStr := utils.FormatAmount(m.statementBalance)
	lastStr := utils.FormatAmount(m.lastReconciledBalance)
	sb.WriteString(fmt.Sprintf("STATEMENT: $%s · %d UNRECONCILED\n", stmtStr, len(m.items)))
	sb.WriteString(fmt.Sprintf("LAST RECONCILED: $%s\n", lastStr))
```

- [ ] **Step 2: Verify build**

Run: `make build`
Expected: exits 0.

- [ ] **Step 3: Run all tests**

Run: `go test ./...`
Expected: all PASS.

- [ ] **Step 4: Commit**

```bash
git add ui/reconcile/model.go
git commit -m "feat(ui): display last reconciled balance and use it in difference formula"
```

---

## Task 9: Final verification

- [ ] **Step 1: Run the full test suite one last time**

Run: `go test ./...`
Expected: all PASS, no failures.

- [ ] **Step 2: Build the binary**

Run: `make build`
Expected: exits 0, `./kea_test` binary produced.

- [ ] **Step 3: Smoke-test non-interactive path compiles correctly**

Run: `./kea_test reconcile --help`
Expected: help text printed with `--balance`, `--ids`, `--force`, `--json` flags visible.
