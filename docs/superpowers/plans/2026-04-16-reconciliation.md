# Reconciliation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement `kea reconcile <account-name>` — an interactive bubbletea TUI that lets the user check off transactions against a bank statement and mark them as reconciled, with a non-interactive flag mode for agent/script use.

**Architecture:** A new `ReconcileEntry` model type carries the data the TUI needs (transaction + split amount for the reconciled account). Two new repository methods handle the SQL: a focused query for unreconciled transactions and a bulk status update. The service method validates IDs, computes the balance difference, and delegates the write to the repo. The bubbletea TUI lives in `ui/reconcile/`, and the cobra command follows the existing `cmd/add.go` + `cmd/add_actions.go` split pattern.

**Tech Stack:** Go, `github.com/charmbracelet/bubbletea` v1.3, `github.com/charmbracelet/lipgloss` v1.1, `github.com/charmbracelet/bubbles` (key), `github.com/charmbracelet/huh` (balance prompt), `github.com/pterm/pterm` (success output), SQLite via `github.com/mattn/go-sqlite3`.

---

## File Map

| Action | File | Responsibility |
|--------|------|----------------|
| Modify | `internal/model/transaction.go` | Add `ReconcileEntry` type |
| Modify | `internal/repository/interfaces.go` | Add 2 new methods to `TransactionRepository` |
| Modify | `internal/service/testhelper_test.go` | Add mock implementations for new repo methods |
| Create | `internal/service/reconcile_ops.go` | `ReconcileTransactions` + `GetUnreconciledByAccount` |
| Create | `internal/service/reconcile_ops_test.go` | Service-layer tests |
| Create | `internal/store/sqlite_reconcile.go` | Store implementations of new repo methods |
| Create | `ui/reconcile/keys.go` | bubbletea key bindings |
| Create | `ui/reconcile/model.go` | bubbletea Model, Init, Update, View |
| Create | `cmd/reconcile.go` | Cobra command definition, flag binding |
| Create | `cmd/reconcile_actions.go` | Runner, Run(), interactive/non-interactive branching |
| Modify | `cmd/root.go` | Register `NewReconcileCmd` |

---

## Task 1: Add `ReconcileEntry` model type

**Files:**
- Modify: `internal/model/transaction.go`

- [ ] **Step 1: Add the type**

Append to the end of `internal/model/transaction.go`:

```go
// ReconcileEntry is a read-only projection used by the reconciliation workflow.
// It carries a transaction plus the net split amount for the queried account,
// so the TUI can display amounts and the service can compute the cleared balance
// without issuing per-transaction queries.
type ReconcileEntry struct {
	ID          int64
	Timestamp   int64
	Description string
	Status      TransactionStatus
	Amount      int64 // net split amount for the reconciled account
}
```

- [ ] **Step 2: Verify the project builds**

```bash
go build ./...
```

Expected: no output (clean build).

- [ ] **Step 3: Commit**

```bash
git add internal/model/transaction.go
git commit -m "feat(model): add ReconcileEntry type for reconciliation workflow"
```

---

## Task 2: Extend `TransactionRepository` interface

**Files:**
- Modify: `internal/repository/interfaces.go`

- [ ] **Step 1: Add the two new method signatures to `TransactionRepository`**

In `internal/repository/interfaces.go`, add the following two lines inside the `TransactionRepository` interface, after the `GetSplitsWithAccountsByTransaction` line:

```go
	// GetUnreconciledTransactionsByAccount returns all Pending and Cleared
	// transactions that have a split touching accountID, together with the
	// net split amount for that account. Ordered by timestamp ASC.
	GetUnreconciledTransactionsByAccount(accountID int64) ([]*model.ReconcileEntry, error)

	// BulkUpdateTransactionStatus sets status on all listed transaction IDs
	// in a single atomic UPDATE. Returns an error if the affected row count
	// does not match len(txIDs).
	BulkUpdateTransactionStatus(txIDs []int64, status model.TransactionStatus) error
```

- [ ] **Step 2: Verify the build fails with expected errors**

```bash
go build ./...
```

Expected: compile errors from `internal/store/` saying `*Store` does not implement `TransactionRepository` (missing the two new methods). This is correct — the store implementations come in Task 6.

- [ ] **Step 3: Commit**

```bash
git add internal/repository/interfaces.go
git commit -m "feat(repository): add GetUnreconciledTransactionsByAccount and BulkUpdateTransactionStatus to interface"
```

---

## Task 3: Extend `mockTransactionRepo` with the two new methods

**Files:**
- Modify: `internal/service/testhelper_test.go`

- [ ] **Step 1: Add fields to `mockTransactionRepo`**

Inside the `mockTransactionRepo` struct (after `updateSplitCalls []int64`), add:

```go
	// reconciliation support
	unreconciledByAccount    map[int64][]*model.ReconcileEntry
	unreconciledByAccountErr map[int64]error
	bulkUpdateErr            error
	bulkUpdateCalls          [][]int64
```

- [ ] **Step 2: Initialize the new maps in `newMockTransactionRepo`**

Inside the struct literal in `newMockTransactionRepo()`, add these two lines alongside the other `make(...)` initializations:

```go
	unreconciledByAccount:    make(map[int64][]*model.ReconcileEntry),
	unreconciledByAccountErr: make(map[int64]error),
```

- [ ] **Step 3: Add the two method implementations**

Append to the end of the `mockTransactionRepo` section in `testhelper_test.go`, before the `mockCombinedRepo` section:

```go
func (m *mockTransactionRepo) GetUnreconciledTransactionsByAccount(accountID int64) ([]*model.ReconcileEntry, error) {
	if err, ok := m.unreconciledByAccountErr[accountID]; ok {
		return nil, err
	}
	return m.unreconciledByAccount[accountID], nil
}

func (m *mockTransactionRepo) BulkUpdateTransactionStatus(txIDs []int64, status model.TransactionStatus) error {
	if m.bulkUpdateErr != nil {
		return m.bulkUpdateErr
	}
	ids := make([]int64, len(txIDs))
	copy(ids, txIDs)
	m.bulkUpdateCalls = append(m.bulkUpdateCalls, ids)
	for _, id := range txIDs {
		if tx, ok := m.transactions[id]; ok {
			tx.Status = status
		}
	}
	return nil
}
```

- [ ] **Step 4: Verify the service tests still pass**

```bash
go test ./internal/service/...
```

Expected: all existing tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/service/testhelper_test.go
git commit -m "test(service): extend mockTransactionRepo with reconciliation methods"
```

---

## Task 4: Write failing service tests

**Files:**
- Create: `internal/service/reconcile_ops_test.go`

- [ ] **Step 1: Create the test file**

```go
package service

import (
	"testing"

	"github.com/hance08/kea/internal/model"
)

// helper: build a ReconcileEntry slice for a given accountID in the mock
func seedUnreconciled(txRepo *mockTransactionRepo, accountID int64, entries []*model.ReconcileEntry) {
	txRepo.unreconciledByAccount[accountID] = entries
}

func TestReconcileTransactions_ZeroDifference(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Checking", Type: model.AccountTypeAsset})
	seedUnreconciled(txRepo, 1, []*model.ReconcileEntry{
		{ID: 10, Timestamp: 1000, Description: "Salary", Status: model.StatusCleared, Amount: 100000},
		{ID: 11, Timestamp: 1001, Description: "Rent", Status: model.StatusCleared, Amount: -50000},
	})

	// statementBalance = 50000 = 100000 + (-50000)
	diff, err := svc.ReconcileTransactions(1, 50000, []int64{10, 11})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diff != 0 {
		t.Errorf("expected difference 0, got %d", diff)
	}
	if len(txRepo.bulkUpdateCalls) != 1 {
		t.Errorf("expected 1 bulk update call, got %d", len(txRepo.bulkUpdateCalls))
	}
}

func TestReconcileTransactions_NonZeroDifference(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Checking", Type: model.AccountTypeAsset})
	seedUnreconciled(txRepo, 1, []*model.ReconcileEntry{
		{ID: 10, Timestamp: 1000, Description: "Salary", Status: model.StatusCleared, Amount: 100000},
	})

	// statementBalance = 120000, but we're only reconciling 100000 → diff = 20000
	diff, err := svc.ReconcileTransactions(1, 120000, []int64{10})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diff != 20000 {
		t.Errorf("expected difference 20000, got %d", diff)
	}
	if len(txRepo.bulkUpdateCalls) != 1 {
		t.Fatalf("expected bulk update to be called")
	}
}

func TestReconcileTransactions_UnknownTxID(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Checking", Type: model.AccountTypeAsset})
	seedUnreconciled(txRepo, 1, []*model.ReconcileEntry{
		{ID: 10, Timestamp: 1000, Description: "Salary", Status: model.StatusCleared, Amount: 100000},
	})

	_, err := svc.ReconcileTransactions(1, 100000, []int64{10, 99}) // 99 is unknown

	if err == nil {
		t.Fatal("expected error for unknown transaction ID, got nil")
	}
	if len(txRepo.bulkUpdateCalls) != 0 {
		t.Error("bulk update must not be called when validation fails")
	}
}

func TestReconcileTransactions_AlreadyReconciledID(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Checking", Type: model.AccountTypeAsset})
	// unreconciledByAccount does NOT contain ID 20 (it's already reconciled)
	seedUnreconciled(txRepo, 1, []*model.ReconcileEntry{
		{ID: 10, Timestamp: 1000, Description: "Salary", Status: model.StatusCleared, Amount: 100000},
	})

	_, err := svc.ReconcileTransactions(1, 100000, []int64{10, 20}) // 20 not in unreconciled set

	if err == nil {
		t.Fatal("expected error for already-reconciled ID, got nil")
	}
	if len(txRepo.bulkUpdateCalls) != 0 {
		t.Error("bulk update must not be called when validation fails")
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

	// account ID 99 does not exist in accRepo

	_, err := svc.ReconcileTransactions(99, 100000, []int64{10})

	if err == nil {
		t.Fatal("expected error for unknown account, got nil")
	}
	if len(txRepo.bulkUpdateCalls) != 0 {
		t.Error("bulk update must not be called when account lookup fails")
	}
}
```

- [ ] **Step 2: Run tests — confirm they all fail**

```bash
go test ./internal/service/... -run TestReconcileTransactions -v
```

Expected: `FAIL` — `svc.ReconcileTransactions undefined` (method does not exist yet).

- [ ] **Step 3: Commit**

```bash
git add internal/service/reconcile_ops_test.go
git commit -m "test(service): add failing tests for ReconcileTransactions"
```

---

## Task 5: Implement service methods

**Files:**
- Create: `internal/service/reconcile_ops.go`

- [ ] **Step 1: Create the implementation file**

```go
package service

import (
	"fmt"

	"github.com/hance08/kea/internal/model"
)

// GetUnreconciledByAccount returns all Pending/Cleared transactions that
// have a split touching the given account, including the split amount for
// that account. Used to populate the reconciliation TUI.
func (ts *TransactionService) GetUnreconciledByAccount(accountID int64) ([]*model.ReconcileEntry, error) {
	entries, err := ts.txRepo.GetUnreconciledTransactionsByAccount(accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to load unreconciled transactions: %w", err)
	}
	return entries, nil
}

// ReconcileTransactions marks the given transaction IDs as reconciled for
// accountID. It validates that every requested ID is in the unreconciled set
// for that account before writing anything.
//
// Returns the difference between statementBalance and the sum of the selected
// split amounts for accountID. A non-zero difference is informational — the
// method always commits if the IDs are valid. The caller decides whether to
// warn (non-zero diff) or proceed silently (zero diff).
func (ts *TransactionService) ReconcileTransactions(accountID int64, statementBalance int64, txIDs []int64) (int64, error) {
	if len(txIDs) == 0 {
		return 0, fmt.Errorf("no transactions selected for reconciliation")
	}

	// 1. Verify account exists.
	if _, err := ts.accRepo.GetAccountByID(accountID); err != nil {
		return 0, fmt.Errorf("account not found: %w", err)
	}

	// 2. Fetch unreconciled transactions and build a valid-ID → amount map.
	entries, err := ts.txRepo.GetUnreconciledTransactionsByAccount(accountID)
	if err != nil {
		return 0, fmt.Errorf("failed to load unreconciled transactions: %w", err)
	}

	validAmounts := make(map[int64]int64, len(entries)) // txID → split amount
	for _, e := range entries {
		validAmounts[e.ID] = e.Amount
	}

	// 3. Validate every requested ID and accumulate the cleared balance.
	var clearedBalance int64
	for _, id := range txIDs {
		amount, ok := validAmounts[id]
		if !ok {
			return 0, fmt.Errorf("transaction ID %d is not in the unreconciled set for this account", id)
		}
		clearedBalance += amount
	}

	// 4. Atomically mark all selected transactions as reconciled.
	if err := ts.txRepo.BulkUpdateTransactionStatus(txIDs, model.StatusReconciled); err != nil {
		return 0, fmt.Errorf("failed to reconcile transactions: %w", err)
	}

	return statementBalance - clearedBalance, nil
}
```

- [ ] **Step 2: Run the tests — confirm they all pass**

```bash
go test ./internal/service/... -run TestReconcileTransactions -v
```

Expected: all 6 tests show `PASS`.

- [ ] **Step 3: Run the full service test suite**

```bash
go test ./internal/service/...
```

Expected: all tests pass, no regressions.

- [ ] **Step 4: Commit**

```bash
git add internal/service/reconcile_ops.go
git commit -m "feat(service): implement ReconcileTransactions and GetUnreconciledByAccount"
```

---

## Task 6: Implement store methods

**Files:**
- Create: `internal/store/sqlite_reconcile.go`

- [ ] **Step 1: Create the implementation file**

```go
package store

import (
	"fmt"
	"strings"

	"github.com/hance08/kea/internal/model"
)

// GetUnreconciledTransactionsByAccount returns all Pending (0) and Cleared (1)
// transactions that have a split on accountID, together with the net split
// amount for that account (grouped to handle rare multi-split-per-account cases).
// Results are ordered by timestamp ASC so the TUI shows chronological order.
func (s *Store) GetUnreconciledTransactionsByAccount(accountID int64) ([]*model.ReconcileEntry, error) {
	rows, err := s.db.Query(`
        SELECT t.id, t.timestamp, t.description, t.status, SUM(s.amount) AS amount
        FROM transactions t
        INNER JOIN splits s ON t.id = s.transaction_id
        WHERE s.account_id = ?
          AND t.status IN (0, 1)
        GROUP BY t.id, t.timestamp, t.description, t.status
        ORDER BY t.timestamp ASC, t.id ASC
    `, accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to query unreconciled transactions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []*model.ReconcileEntry
	for rows.Next() {
		e := &model.ReconcileEntry{}
		if err := rows.Scan(&e.ID, &e.Timestamp, &e.Description, &e.Status, &e.Amount); err != nil {
			return nil, fmt.Errorf("failed to scan reconcile entry: %w", err)
		}
		result = append(result, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}
	return result, nil
}

// BulkUpdateTransactionStatus sets the status of all listed transaction IDs
// in a single UPDATE statement. Returns an error if the affected row count
// does not match len(txIDs) — indicating one or more IDs did not exist.
func (s *Store) BulkUpdateTransactionStatus(txIDs []int64, status model.TransactionStatus) error {
	if len(txIDs) == 0 {
		return nil
	}

	placeholders := strings.Repeat("?,", len(txIDs))
	placeholders = placeholders[:len(placeholders)-1] // trim trailing comma

	query := fmt.Sprintf(
		"UPDATE transactions SET status = ? WHERE id IN (%s)",
		placeholders,
	)

	args := make([]any, 0, len(txIDs)+1)
	args = append(args, status)
	for _, id := range txIDs {
		args = append(args, id)
	}

	result, err := s.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("failed to bulk update transaction status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected != int64(len(txIDs)) {
		return fmt.Errorf("expected to update %d transactions, updated %d", len(txIDs), rowsAffected)
	}
	return nil
}
```

- [ ] **Step 2: Verify the project builds cleanly (store now satisfies the interface)**

```bash
go build ./...
```

Expected: no output (clean build).

- [ ] **Step 3: Run all tests**

```bash
go test ./...
```

Expected: all tests pass.

- [ ] **Step 4: Commit**

```bash
git add internal/store/sqlite_reconcile.go
git commit -m "feat(store): implement GetUnreconciledTransactionsByAccount and BulkUpdateTransactionStatus"
```

---

## Task 7: TUI key bindings

**Files:**
- Create: `ui/reconcile/keys.go`

- [ ] **Step 1: Create the keys file**

```go
package reconcileui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	Up      key.Binding
	Down    key.Binding
	Toggle  key.Binding
	Confirm key.Binding
	Quit    key.Binding
	Yes     key.Binding
	No      key.Binding
}

func defaultKeyMap() keyMap {
	return keyMap{
		Up:      key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		Down:    key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
		Toggle:  key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "toggle")),
		Confirm: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "finish")),
		Quit:    key.NewBinding(key.WithKeys("q", "esc"), key.WithHelp("q/esc", "quit")),
		Yes:     key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "confirm")),
		No:      key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "cancel")),
	}
}
```

- [ ] **Step 2: Verify it builds**

```bash
go build ./ui/reconcile/...
```

Expected: no output.

- [ ] **Step 3: Commit**

```bash
git add ui/reconcile/keys.go
git commit -m "feat(ui/reconcile): add bubbletea key bindings"
```

---

## Task 8: TUI model

**Files:**
- Create: `ui/reconcile/model.go`

- [ ] **Step 1: Create the model file**

```go
package reconcileui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/utils"
)

type listItem struct {
	entry   *model.ReconcileEntry
	checked bool
}

// Model is the bubbletea model for the reconciliation TUI.
// After tea.Program.Run() returns, inspect Cancelled() and SelectedIDs()
// to determine the outcome.
type Model struct {
	accountName      string
	statementBalance int64
	items            []listItem
	cursor           int
	confirmPending   bool // waiting for y/n after Enter with non-zero diff
	cancelled        bool
	done             bool
	keys             keyMap
}

// NewModel constructs the initial reconciliation model.
func NewModel(accountName string, statementBalance int64, entries []*model.ReconcileEntry) Model {
	items := make([]listItem, len(entries))
	for i, e := range entries {
		items[i] = listItem{entry: e}
	}
	return Model{
		accountName:      accountName,
		statementBalance: statementBalance,
		items:            items,
		keys:             defaultKeyMap(),
	}
}

// Cancelled reports whether the user quit without confirming.
func (m Model) Cancelled() bool { return m.cancelled }

// SelectedIDs returns the transaction IDs the user checked off.
func (m Model) SelectedIDs() []int64 {
	var ids []int64
	for _, it := range m.items {
		if it.checked {
			ids = append(ids, it.entry.ID)
		}
	}
	return ids
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.confirmPending {
			switch {
			case key.Matches(msg, m.keys.Yes):
				m.done = true
				return m, tea.Quit
			case key.Matches(msg, m.keys.No):
				m.confirmPending = false
			}
			return m, nil
		}
		switch {
		case key.Matches(msg, m.keys.Quit):
			m.cancelled = true
			return m, tea.Quit
		case key.Matches(msg, m.keys.Up):
			if m.cursor > 0 {
				m.cursor--
			}
		case key.Matches(msg, m.keys.Down):
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		case key.Matches(msg, m.keys.Toggle):
			if len(m.items) > 0 {
				m.items[m.cursor].checked = !m.items[m.cursor].checked
			}
		case key.Matches(msg, m.keys.Confirm):
			if len(m.items) == 0 {
				return m, nil
			}
			if m.difference() != 0 {
				m.confirmPending = true
			} else {
				m.done = true
				return m, tea.Quit
			}
		}
	}
	return m, nil
}

func (m Model) View() string {
	if len(m.items) == 0 {
		return "No unreconciled transactions for this account.\n\nPress q to quit.\n"
	}

	var sb strings.Builder

	// ── Header ──────────────────────────────────────────
	diff := m.difference()
	var badge string
	if diff == 0 {
		badge = lipgloss.NewStyle().
			Padding(0, 1).
			Background(lipgloss.Color("22")).
			Foreground(lipgloss.Color("15")).
			Render("BALANCED")
	} else {
		badge = lipgloss.NewStyle().
			Padding(0, 1).
			Background(lipgloss.Color("58")).
			Foreground(lipgloss.Color("15")).
			Render(fmt.Sprintf("OFF BY $%s", utils.FormatAmount(abs(diff))))
	}

	accountStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("78"))
	sb.WriteString(fmt.Sprintf("%-40s %s\n\n", accountStyle.Render(m.accountName), badge))

	stmtStr := utils.FormatAmount(m.statementBalance)
	sb.WriteString(fmt.Sprintf("STATEMENT: $%s · %d UNRECONCILED\n", stmtStr, len(m.items)))
	sb.WriteString(strings.Repeat("─", 52) + "\n")

	// ── Transaction list ────────────────────────────────
	for i, it := range m.items {
		checkbox := "[ ]"
		rowStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
		if it.checked {
			checkbox = "[✓]"
			rowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("78"))
		}

		date := time.Unix(it.entry.Timestamp, 0).Format("Jan 02")
		amt := fmt.Sprintf("$%s", utils.FormatAmount(it.entry.Amount))
		line := fmt.Sprintf("%s %s  %-28s %10s", checkbox, date, truncate(it.entry.Description, 28), amt)

		if i == m.cursor {
			line = lipgloss.NewStyle().
				Background(lipgloss.Color("236")).
				Foreground(lipgloss.Color("255")).
				Render(line)
		} else {
			line = rowStyle.Render(line)
		}
		sb.WriteString(line + "\n")
	}

	sb.WriteString(strings.Repeat("─", 52) + "\n")

	// ── Footer balance ───────────────────────────────────
	clearedStr := utils.FormatAmount(m.clearedBalance())
	diffStr := utils.FormatAmount(abs(diff))
	remainingStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	if diff == 0 {
		remainingStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("78"))
	}
	sb.WriteString(fmt.Sprintf(
		"Cleared $%s · %s\n",
		clearedStr,
		remainingStyle.Render(fmt.Sprintf("Remaining $%s", diffStr)),
	))

	// ── Prompt or hint ───────────────────────────────────
	if m.confirmPending {
		warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
		sb.WriteString(fmt.Sprintf("\n%s\n",
			warnStyle.Render(fmt.Sprintf("You're off by $%s. Confirm anyway? (y/n)", diffStr)),
		))
	} else {
		sb.WriteString("\nspace toggle · enter finish · ↑↓ navigate · q quit\n")
	}

	return sb.String()
}

// ── helpers ─────────────────────────────────────────────

func (m Model) clearedBalance() int64 {
	var total int64
	for _, it := range m.items {
		if it.checked {
			total += it.entry.Amount
		}
	}
	return total
}

func (m Model) difference() int64 {
	return m.statementBalance - m.clearedBalance()
}

func abs(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}
```

- [ ] **Step 2: Verify it builds**

```bash
go build ./ui/reconcile/...
```

Expected: no output.

- [ ] **Step 3: Commit**

```bash
git add ui/reconcile/model.go
git commit -m "feat(ui/reconcile): implement bubbletea reconciliation TUI model"
```

---

## Task 9: Cobra command definition

**Files:**
- Create: `cmd/reconcile.go`

- [ ] **Step 1: Create the command file**

```go
package cmd

import (
	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/service"
	"github.com/spf13/cobra"
)

// reconcileAccountProvider is the subset of AccountService used by the runner.
type reconcileAccountProvider interface {
	GetAccountByName(name string) (*model.Account, error)
}

// reconcileTxProvider is the subset of TransactionService used by the runner.
type reconcileTxProvider interface {
	GetUnreconciledByAccount(accountID int64) ([]*model.ReconcileEntry, error)
	ReconcileTransactions(accountID int64, statementBalance int64, txIDs []int64) (int64, error)
}

type reconcileFlags struct {
	Balance string
	IDs     string
	Force   bool
	JSON    bool
}

type reconcileRunner struct {
	accSvc reconcileAccountProvider
	txSvc  reconcileTxProvider
	flags  *reconcileFlags
}

// NewReconcileCmd wires up the `kea reconcile` command.
func NewReconcileCmd(svc *service.Service) *cobra.Command {
	flags := &reconcileFlags{}

	cmd := &cobra.Command{
		Use:   "reconcile <account-name>",
		Short: "Reconcile an account against a statement balance",
		Long: `Compare your records against an external statement and mark
matching transactions as reconciled.

Interactive mode (default):
  kea reconcile "Assets:Checking"

Non-interactive / agent mode (all flags required):
  kea reconcile "Assets:Checking" --balance 2450.00 --ids 12,15,18 --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runner := &reconcileRunner{
				accSvc: svc.Account(),
				txSvc:  svc.Transaction(),
				flags:  flags,
			}
			return runner.Run(cmd, args)
		},
	}

	cmd.Flags().StringVar(&flags.Balance, "balance", "", "statement ending balance (e.g. 2450.00)")
	cmd.Flags().StringVar(&flags.IDs, "ids", "", "comma-separated transaction IDs to reconcile (non-interactive)")
	cmd.Flags().BoolVar(&flags.Force, "force", false, "skip balance-mismatch warning (non-interactive mode)")
	cmd.Flags().BoolVarP(&flags.JSON, "json", "j", false, "output result as JSON")

	return cmd
}
```

- [ ] **Step 2: Verify it builds**

```bash
go build ./cmd/...
```

Expected: no output.

- [ ] **Step 3: Commit**

```bash
git add cmd/reconcile.go
git commit -m "feat(cmd): add reconcile command definition and provider interfaces"
```

---

## Task 10: Command runner (interactive + non-interactive)

**Files:**
- Create: `cmd/reconcile_actions.go`

- [ ] **Step 1: Create the actions file**

```go
package cmd

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/utils"
	"github.com/hance08/kea/ui/prompts"
	reconcileui "github.com/hance08/kea/ui/reconcile"
	"github.com/hance08/kea/ui/views"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

func (r *reconcileRunner) Run(cmd *cobra.Command, args []string) error {
	accountName := args[0]

	// Resolve account.
	acc, err := r.accSvc.GetAccountByName(accountName)
	if err != nil {
		return fmt.Errorf("account %q not found: %w", accountName, err)
	}

	nonInteractive := cmd.Flags().Changed("balance") && cmd.Flags().Changed("ids")

	if nonInteractive {
		return r.runNonInteractive(acc)
	}
	return r.runInteractive(acc)
}

// ── Non-interactive (agent / script) mode ────────────────────────────────────

func (r *reconcileRunner) runNonInteractive(acc *model.Account) error {
	statementBalance, err := utils.ParseAmount(r.flags.Balance)
	if err != nil {
		return fmt.Errorf("invalid --balance value %q: %w", r.flags.Balance, err)
	}

	txIDs, err := parseIDs(r.flags.IDs)
	if err != nil {
		return fmt.Errorf("invalid --ids value %q: %w", r.flags.IDs, err)
	}

	diff, err := r.txSvc.ReconcileTransactions(acc.ID, statementBalance, txIDs)
	if err != nil {
		return err
	}

	if diff != 0 && !r.flags.Force {
		return fmt.Errorf(
			"balance mismatch: off by $%s — use --force to reconcile anyway",
			utils.FormatAmount(abs64(diff)),
		)
	}

	if r.flags.JSON {
		return writeReconcileJSON(acc.Name, len(txIDs), diff)
	}
	pterm.Success.Printf(
		"Reconciled %d transaction(s) on %q. Difference: $%s\n",
		len(txIDs), acc.Name, utils.FormatAmount(abs64(diff)),
	)
	return nil
}

// ── Interactive mode ──────────────────────────────────────────────────────────

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

	// 2. Load unreconciled entries.
	entries, err := r.txSvc.GetUnreconciledByAccount(acc.ID)
	if err != nil {
		return err
	}

	// 3. Run bubbletea TUI.
	m := reconcileui.NewModel(acc.Name, statementBalance, entries)
	prog := tea.NewProgram(m)
	finalRaw, err := prog.Run()
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}
	finalModel := finalRaw.(reconcileui.Model)

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

// ── helpers ───────────────────────────────────────────────────────────────────

func parseIDs(s string) ([]int64, error) {
	if strings.TrimSpace(s) == "" {
		return nil, fmt.Errorf("empty ID list")
	}
	parts := strings.Split(s, ",")
	ids := make([]int64, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		id, err := strconv.ParseInt(p, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid ID %q: %w", p, err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func writeReconcileJSON(account string, count int, diff int64) error {
	return views.WriteJSON(map[string]any{
		"account":          account,
		"reconciled_count": count,
		"difference":       diff,
	})
}

func abs64(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}
```

- [ ] **Step 2: Verify it builds**

```bash
go build ./cmd/...
```

Expected: no output.

- [ ] **Step 3: Commit**

```bash
git add cmd/reconcile_actions.go
git commit -m "feat(cmd): implement reconcile runner with interactive and non-interactive modes"
```

---

## Task 11: Wire the command in `root.go`

**Files:**
- Modify: `cmd/root.go`

- [ ] **Step 1: Register `NewReconcileCmd`**

In `cmd/root.go`, inside the `exitCode` closure where the other top-level commands are registered (around line 119–123), add one line:

```go
rootCmd.AddCommand(NewReconcileCmd(application.Service))
```

Place it after `rootCmd.AddCommand(NewReportCmd(application.Service))` so the ordering is consistent.

- [ ] **Step 2: Build and smoke-test**

```bash
make build
./kea_test reconcile --help
```

Expected output:
```
Compare your records against an external statement and mark
matching transactions as reconciled.
...
Usage:
  kea reconcile <account-name> [flags]

Flags:
      --balance string   statement ending balance (e.g. 2450.00)
      --force            skip balance-mismatch warning (non-interactive mode)
  -h, --help             help for reconcile
      --ids string       comma-separated transaction IDs to reconcile (non-interactive)
  -j, --json             output result as JSON
```

- [ ] **Step 3: Run the full test suite**

```bash
go test ./...
```

Expected: all tests pass.

- [ ] **Step 4: Commit**

```bash
git add cmd/root.go
git commit -m "feat(cmd): register kea reconcile command in root"
```

---

## Task 12: Final smoke test

- [ ] **Step 1: Build**

```bash
make build
```

Expected: produces `./kea_test`, no errors.

- [ ] **Step 2: Verify the command is listed in help**

```bash
./kea_test --help
```

Expected: `reconcile` appears in the command list.

- [ ] **Step 3: Verify non-interactive mode produces JSON**

```bash
./kea_test reconcile "Assets:Checking" --balance 0 --ids 1 --force --json 2>&1 || true
```

Expected: either a JSON result or an error message (e.g. account not found) — the key is no panic and structured output.

- [ ] **Step 4: Commit if clean**

If there are any lint or build issues found during smoke testing, fix them and commit with:

```bash
git commit -m "fix(reconcile): address smoke test issues"
```
