# Bulk Transaction Details Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Eliminate the N+1 query problem in the transaction list command by fetching all split details in a single bulk query.

**Architecture:** Add `GetSplitsWithAccountsByTransactionIDs` to the repository/store layer (single SQL query with IN clause), expose `GetTransactionDetailsByIDs` from the service layer, and rewire `buildViewItems` in the command layer to call it once instead of looping.

**Tech Stack:** Go, SQLite, existing repository/service/cmd architecture.

---

## File Map

| Action | File | Responsibility |
|--------|------|----------------|
| Modify | `internal/repository/interfaces.go:28` | Add new method to `TransactionRepository` |
| Modify | `internal/store/sqlite_transaction.go:365` | Implement bulk query |
| Modify | `internal/service/transaction_service.go:51` | Add `GetTransactionDetailsByIDs` |
| Modify | `internal/service/testhelper_test.go:408` | Add mock for new repo method |
| Create | `internal/service/transaction_service_bulk_test.go` | Test for `GetTransactionDetailsByIDs` |
| Modify | `cmd/transaction/list.go:19-28` | Update interface + rewrite `buildViewItems` |

---

### Task 1: Add Repository Interface Method

**Files:**
- Modify: `internal/repository/interfaces.go:47` (after `GetSplitsWithAccountsByTransaction`)

- [ ] **Step 1: Add the new method signature**

In `internal/repository/interfaces.go`, add after line 51 (after `GetSplitsWithAccountsByTransaction`):

```go
// GetSplitsWithAccountsByTransactionIDs returns all splits (with account info)
// for the given transaction IDs in a single query, keyed by transaction ID.
GetSplitsWithAccountsByTransactionIDs(ctx context.Context, txIDs []int64) (map[int64][]model.SplitDetail, error)
```

- [ ] **Step 2: Verify the project compiles (expect failures in store/mock)**

Run: `go build ./internal/repository/...`
Expected: PASS (interfaces compile independently)

Run: `go build ./...`
Expected: FAIL — `Store` and mock don't implement the new method yet.

- [ ] **Step 3: Commit**

```bash
git add internal/repository/interfaces.go
git commit -m "feat(repository): add GetSplitsWithAccountsByTransactionIDs interface method"
```

---

### Task 2: Implement Store Method

**Files:**
- Modify: `internal/store/sqlite_transaction.go:365` (after `GetSplitsWithAccountsByTransaction`)

- [ ] **Step 1: Implement the bulk query method**

Add after `GetSplitsWithAccountsByTransaction` (after line 365):

```go
func (s *Store) GetSplitsWithAccountsByTransactionIDs(ctx context.Context, txIDs []int64) (map[int64][]model.SplitDetail, error) {
	if len(txIDs) == 0 {
		return make(map[int64][]model.SplitDetail), nil
	}

	placeholders := make([]byte, 0, len(txIDs)*2-1)
	args := make([]any, len(txIDs))
	for i, id := range txIDs {
		if i > 0 {
			placeholders = append(placeholders, ',')
		}
		placeholders = append(placeholders, '?')
		args[i] = id
	}

	query := `
        SELECT
            s.id, s.transaction_id, s.account_id, s.amount, s.currency, s.memo,
            a.name, a.type
        FROM splits s
        JOIN accounts a ON s.account_id = a.id
        WHERE s.transaction_id IN (` + string(placeholders) + `)
        ORDER BY s.transaction_id, s.id`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query splits by transaction IDs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := make(map[int64][]model.SplitDetail)
	for rows.Next() {
		var d model.SplitDetail
		var txID int64
		if err := rows.Scan(
			&d.ID, &txID, &d.AccountID, &d.Amount, &d.Currency, &d.Memo,
			&d.AccountName, &d.AccountType,
		); err != nil {
			return nil, fmt.Errorf("failed to scan split with account: %w", err)
		}
		result[txID] = append(result[txID], d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}
	return result, nil
}
```

- [ ] **Step 2: Verify store compiles**

Run: `go build ./internal/store/...`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/store/sqlite_transaction.go
git commit -m "feat(store): implement GetSplitsWithAccountsByTransactionIDs bulk query"
```

---

### Task 3: Add Mock Method for Service Tests

**Files:**
- Modify: `internal/service/testhelper_test.go`

- [ ] **Step 1: Add injectable field to mockTransactionRepo**

In the `mockTransactionRepo` struct (around line 222), add a new field after `splitsRangeErr`:

```go
// default return for GetSplitsWithAccountsByTransactionIDs
splitsByTxIDsErr error
```

- [ ] **Step 2: Implement the mock method**

Add after `GetSplitsWithAccountsByTransaction` (after line 421):

```go
func (m *mockTransactionRepo) GetSplitsWithAccountsByTransactionIDs(_ context.Context, txIDs []int64) (map[int64][]model.SplitDetail, error) {
	if m.splitsByTxIDsErr != nil {
		return nil, m.splitsByTxIDsErr
	}
	result := make(map[int64][]model.SplitDetail)
	for _, id := range txIDs {
		if splits, ok := m.splitsWithAccts[id]; ok {
			result[id] = splits
		}
	}
	return result, nil
}
```

This reuses the existing `splitsWithAccts` map (already used for date-range queries) — test setup populates it with txID→splits mappings.

- [ ] **Step 3: Verify project compiles**

Run: `go build ./...`
Expected: FAIL — service layer doesn't have `GetTransactionDetailsByIDs` yet, but store/mock compile.

Run: `go build ./internal/service/...`
Expected: PASS (mock satisfies interface within package)

Actually run: `go vet ./internal/service/...`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/service/testhelper_test.go
git commit -m "test(service): add mock for GetSplitsWithAccountsByTransactionIDs"
```

---

### Task 4: Add Service Method with TDD

**Files:**
- Create: `internal/service/transaction_service_bulk_test.go`
- Modify: `internal/service/transaction_service.go:71` (after `GetTransactionByID`)

- [ ] **Step 1: Write failing test**

Create `internal/service/transaction_service_bulk_test.go`:

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/hance08/kea/internal/model"
)

func TestGetTransactionDetailsByIDs(t *testing.T) {
	t.Run("returns details for multiple transactions", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		svc := newTestTransactionService(accRepo, txRepo)

		txRepo.splitsWithAccts[1] = []model.SplitDetail{
			{ID: 10, AccountID: 1, AccountName: "Assets:Bank", AccountType: "A", Amount: 100, Currency: "USD"},
			{ID: 11, AccountID: 2, AccountName: "Expenses:Food", AccountType: "E", Amount: -100, Currency: "USD"},
		}
		txRepo.splitsWithAccts[2] = []model.SplitDetail{
			{ID: 20, AccountID: 1, AccountName: "Assets:Bank", AccountType: "A", Amount: -50, Currency: "USD"},
			{ID: 21, AccountID: 3, AccountName: "Revenue:Salary", AccountType: "R", Amount: 50, Currency: "USD"},
		}

		txs := []*model.Transaction{
			{ID: 1, Timestamp: 1000, Description: "Lunch", Status: model.StatusCleared, Type: model.TxTypeExpense},
			{ID: 2, Timestamp: 2000, Description: "Pay", Status: model.StatusPending, Type: model.TxTypeIncome},
		}

		result, err := svc.GetTransactionDetailsByIDs(context.Background(), txs)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(result) != 2 {
			t.Fatalf("expected 2 details, got %d", len(result))
		}

		detail1 := result[1]
		if detail1.Description != "Lunch" {
			t.Errorf("expected description 'Lunch', got %q", detail1.Description)
		}
		if len(detail1.Splits) != 2 {
			t.Errorf("expected 2 splits for tx 1, got %d", len(detail1.Splits))
		}

		detail2 := result[2]
		if detail2.Type != model.TxTypeIncome {
			t.Errorf("expected type Income, got %v", detail2.Type)
		}
	})

	t.Run("empty input returns empty map", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		svc := newTestTransactionService(accRepo, txRepo)

		result, err := svc.GetTransactionDetailsByIDs(context.Background(), nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result) != 0 {
			t.Fatalf("expected empty map, got %d entries", len(result))
		}
	})

	t.Run("repo error propagates", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		svc := newTestTransactionService(accRepo, txRepo)

		txRepo.splitsByTxIDsErr = errors.New("db failure")

		txs := []*model.Transaction{
			{ID: 1, Timestamp: 1000, Description: "Test", Status: model.StatusPending, Type: model.TxTypeExpense},
		}

		_, err := svc.GetTransactionDetailsByIDs(context.Background(), txs)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/service/ -run TestGetTransactionDetailsByIDs -v`
Expected: FAIL — `svc.GetTransactionDetailsByIDs` does not exist.

- [ ] **Step 3: Implement the service method**

In `internal/service/transaction_service.go`, add after `GetTransactionByID` (after line 71):

```go
// GetTransactionDetailsByIDs assembles TransactionDetail objects for all given
// transactions in a single bulk query, avoiding N+1 patterns.
func (ts *TransactionService) GetTransactionDetailsByIDs(ctx context.Context, txs []*model.Transaction) (map[int64]*model.TransactionDetail, error) {
	if len(txs) == 0 {
		return make(map[int64]*model.TransactionDetail), nil
	}

	ids := make([]int64, len(txs))
	for i, tx := range txs {
		ids[i] = tx.ID
	}

	splitsMap, err := ts.txRepo.GetSplitsWithAccountsByTransactionIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("failed to bulk-fetch splits: %w", err)
	}

	result := make(map[int64]*model.TransactionDetail, len(txs))
	for _, tx := range txs {
		result[tx.ID] = &model.TransactionDetail{
			ID:          tx.ID,
			Timestamp:   tx.Timestamp,
			Description: tx.Description,
			Status:      tx.Status,
			Type:        tx.Type,
			Splits:      splitsMap[tx.ID],
		}
	}
	return result, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/service/ -run TestGetTransactionDetailsByIDs -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/service/transaction_service.go internal/service/transaction_service_bulk_test.go
git commit -m "feat(service): add GetTransactionDetailsByIDs bulk method"
```

---

### Task 5: Rewire Command Layer

**Files:**
- Modify: `cmd/transaction/list.go`

- [ ] **Step 1: Update ListProvider interface**

Replace the `ListProvider` interface (lines 24-31) with:

```go
type ListProvider interface {
	GetTransactionHistory(ctx context.Context, accountName string, limit int) ([]*model.Transaction, error)
	GetRecentTransactions(ctx context.Context, limit int) ([]*model.Transaction, error)
	GetTransactionDetailsByIDs(ctx context.Context, txs []*model.Transaction) (map[int64]*model.TransactionDetail, error)
	GetDisplayAccount(ctx context.Context, splits []model.SplitDetail, txType string) (string, error)
	GetDisplayOffsetAccount(ctx context.Context, splits []model.SplitDetail, txType string, primaryAccount string) (string, error)
	GetDisplayAmount(splits []model.SplitDetail) (int64, string)
}
```

- [ ] **Step 2: Rewrite buildViewItems**

Replace `buildViewItems` (lines 101-116) with:

```go
func (r *listRunner) buildViewItems(ctx context.Context, transactions []*model.Transaction) []views.TransactionListItem {
	detailsMap, err := r.svc.GetTransactionDetailsByIDs(ctx, transactions)
	if err != nil {
		if !r.flags.JSON {
			r.view.ShowWarning("Failed to load transaction details: %v\n", err)
		}
		return nil
	}

	var viewItems []views.TransactionListItem
	for _, tx := range transactions {
		detail, ok := detailsMap[tx.ID]
		if !ok {
			if !r.flags.JSON {
				r.view.ShowWarning("Skipping transaction %d: no details found\n", tx.ID)
			}
			continue
		}
		viewItems = append(viewItems, r.convertToViewItem(ctx, tx, detail))
	}
	return viewItems
}
```

- [ ] **Step 3: Remove the TODO comment**

Remove line 17: `//TODO: Efficiency optimize`

- [ ] **Step 4: Verify project compiles and tests pass**

Run: `go build ./...`
Expected: PASS

Run: `go test ./...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add cmd/transaction/list.go
git commit -m "perf(cmd): eliminate N+1 query in transaction list (closes #58)"
```

---

### Task 6: Final Verification

- [ ] **Step 1: Run full test suite**

Run: `go test ./... -count=1`
Expected: All PASS

- [ ] **Step 2: Build the binary**

Run: `make build`
Expected: PASS, produces `./kea_test` binary

- [ ] **Step 3: Verify no regressions with vet/lint**

Run: `go vet ./...`
Expected: No issues
