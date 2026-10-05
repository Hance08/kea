# Multi-Dimensional Transaction Filtering Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `TransactionFilter` struct and unified query method so transactions can be filtered by any combination of account, type, status, date range, and description text search — with pagination.

**Architecture:** A new `TransactionFilter` struct (all pointer fields, nil = no filter) is passed alongside `ListOptions` to a single `FilterTransactions` method that builds a dynamic SQL WHERE clause. The store layer constructs the query; the service layer resolves account name → ID before delegating. The CLI `transaction list` command gains `--status`, `--type`, `--from`, `--to`, and `--description` flags.

**Tech Stack:** Go, SQLite, cobra (CLI), pterm (display)

---

## File Map

| Action | File | Responsibility |
|--------|------|---------------|
| Create | `internal/model/transaction_filter.go` | `TransactionFilter` struct |
| Modify | `internal/repository/interfaces.go:28-91` | Add `FilterTransactions` to `TransactionRepository` |
| Modify | `internal/store/sqlite_transaction.go:465+` | Implement `FilterTransactions` with dynamic SQL |
| Modify | `internal/service/transaction_service.go:116-128` | Add `FilterTransactions` service method |
| Modify | `internal/service/testhelper_test.go:261+` | Add `FilterTransactions` to mock |
| Create | `internal/service/transaction_filter_test.go` | Service-layer filter tests |
| Create | `internal/store/sqlite_transaction_filter_test.go` | Store integration tests |
| Modify | `cmd/transaction/list.go` | Add CLI filter flags, switch to `FilterTransactions` |

---

### Task 1: Define TransactionFilter Struct

**Files:**
- Create: `internal/model/transaction_filter.go`

- [ ] **Step 1: Create the filter struct**

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package model

type TransactionFilter struct {
	AccountID   *int64
	Type        *TransactionType
	Status      *TransactionStatus
	StartTime   *int64
	EndTime     *int64
	Description *string
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/model/...`
Expected: success, no errors

- [ ] **Step 3: Commit**

```bash
git add internal/model/transaction_filter.go
git commit -m "feat(model): add TransactionFilter struct (#114)"
```

---

### Task 2: Add FilterTransactions to Repository Interface

**Files:**
- Modify: `internal/repository/interfaces.go:84-91`

- [ ] **Step 1: Add the method to TransactionRepository**

Add after the `ListTransactionsByAccount` method (line 90), before the closing brace of `TransactionRepository`:

```go
	// FilterTransactions returns a paginated list of transactions matching the
	// given filter criteria. Nil filter fields are ignored (no filtering on
	// that dimension). When filter.AccountID is set the query joins on splits.
	// Results are ordered by timestamp DESC, id DESC.
	FilterTransactions(ctx context.Context, filter model.TransactionFilter, opts model.ListOptions) (*model.ListResult[*model.Transaction], error)
```

- [ ] **Step 2: Verify the interface compiles (expect build failures in implementors)**

Run: `go build ./internal/repository/...`
Expected: success (interfaces compile independently)

Run: `go build ./...`
Expected: FAIL — `Store` and mocks don't implement `FilterTransactions` yet

- [ ] **Step 3: Commit**

```bash
git add internal/repository/interfaces.go
git commit -m "feat(repository): add FilterTransactions to TransactionRepository (#114)"
```

---

### Task 3: Implement FilterTransactions in Store

**Files:**
- Modify: `internal/store/sqlite_transaction.go` (after `ListTransactionsByAccount`, ~line 558)

- [ ] **Step 1: Write the store integration test**

Create file `internal/store/sqlite_transaction_filter_test.go`:

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package store

import (
	"context"
	"testing"

	"github.com/hance08/kea/internal/model"
)

func TestFilterTransactions(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Seed accounts
	accBank, _ := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
	accFood, _ := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
	accSalary, _ := s.CreateAccount(ctx, "Revenue:Salary", model.AccountTypeRevenue, "USD", "", nil)

	// Seed transactions
	// tx1: Expense, Cleared, timestamp 1000
	s.CreateTransactionWithSplits(ctx, model.Transaction{
		Timestamp: 1000, Description: "Groceries", Status: model.StatusCleared, Type: model.TxTypeExpense,
	}, []model.Split{
		{AccountID: accBank, Amount: -5000, Currency: "USD"},
		{AccountID: accFood, Amount: 5000, Currency: "USD"},
	})
	// tx2: Income, Pending, timestamp 2000
	s.CreateTransactionWithSplits(ctx, model.Transaction{
		Timestamp: 2000, Description: "Paycheck Jan", Status: model.StatusPending, Type: model.TxTypeIncome,
	}, []model.Split{
		{AccountID: accBank, Amount: 100000, Currency: "USD"},
		{AccountID: accSalary, Amount: -100000, Currency: "USD"},
	})
	// tx3: Expense, Cleared, timestamp 3000
	s.CreateTransactionWithSplits(ctx, model.Transaction{
		Timestamp: 3000, Description: "Restaurant dinner", Status: model.StatusCleared, Type: model.TxTypeExpense,
	}, []model.Split{
		{AccountID: accBank, Amount: -3000, Currency: "USD"},
		{AccountID: accFood, Amount: 3000, Currency: "USD"},
	})

	t.Run("no filters returns all", func(t *testing.T) {
		result, err := s.FilterTransactions(ctx, model.TransactionFilter{}, model.ListOptions{Limit: 10, IncludeCount: true})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.TotalCount != 3 {
			t.Errorf("expected 3 total, got %d", result.TotalCount)
		}
		if len(result.Items) != 3 {
			t.Errorf("expected 3 items, got %d", len(result.Items))
		}
	})

	t.Run("filter by account", func(t *testing.T) {
		result, err := s.FilterTransactions(ctx, model.TransactionFilter{AccountID: &accFood}, model.ListOptions{Limit: 10, IncludeCount: true})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.TotalCount != 2 {
			t.Errorf("expected 2, got %d", result.TotalCount)
		}
	})

	t.Run("filter by type", func(t *testing.T) {
		txType := model.TxTypeIncome
		result, err := s.FilterTransactions(ctx, model.TransactionFilter{Type: &txType}, model.ListOptions{Limit: 10, IncludeCount: true})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.TotalCount != 1 {
			t.Errorf("expected 1, got %d", result.TotalCount)
		}
		if result.Items[0].Description != "Paycheck Jan" {
			t.Errorf("expected Paycheck Jan, got %s", result.Items[0].Description)
		}
	})

	t.Run("filter by status", func(t *testing.T) {
		status := model.StatusCleared
		result, err := s.FilterTransactions(ctx, model.TransactionFilter{Status: &status}, model.ListOptions{Limit: 10, IncludeCount: true})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.TotalCount != 2 {
			t.Errorf("expected 2, got %d", result.TotalCount)
		}
	})

	t.Run("filter by date range", func(t *testing.T) {
		start := int64(1500)
		end := int64(2500)
		result, err := s.FilterTransactions(ctx, model.TransactionFilter{StartTime: &start, EndTime: &end}, model.ListOptions{Limit: 10, IncludeCount: true})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.TotalCount != 1 {
			t.Errorf("expected 1, got %d", result.TotalCount)
		}
	})

	t.Run("filter by description", func(t *testing.T) {
		desc := "dinner"
		result, err := s.FilterTransactions(ctx, model.TransactionFilter{Description: &desc}, model.ListOptions{Limit: 10, IncludeCount: true})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.TotalCount != 1 {
			t.Errorf("expected 1, got %d", result.TotalCount)
		}
		if result.Items[0].Description != "Restaurant dinner" {
			t.Errorf("expected Restaurant dinner, got %s", result.Items[0].Description)
		}
	})

	t.Run("combined filters", func(t *testing.T) {
		status := model.StatusCleared
		txType := model.TxTypeExpense
		result, err := s.FilterTransactions(ctx, model.TransactionFilter{
			AccountID: &accFood,
			Status:    &status,
			Type:      &txType,
		}, model.ListOptions{Limit: 10, IncludeCount: true})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.TotalCount != 2 {
			t.Errorf("expected 2, got %d", result.TotalCount)
		}
	})

	t.Run("pagination with filter", func(t *testing.T) {
		status := model.StatusCleared
		page1, err := s.FilterTransactions(ctx, model.TransactionFilter{Status: &status}, model.ListOptions{Limit: 1, Offset: 0, IncludeCount: true})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(page1.Items) != 1 {
			t.Fatalf("expected 1 item, got %d", len(page1.Items))
		}
		if page1.TotalCount != 2 {
			t.Errorf("expected total 2, got %d", page1.TotalCount)
		}
		// page1 should be the newest (timestamp 3000)
		if page1.Items[0].Timestamp != 3000 {
			t.Errorf("expected timestamp 3000, got %d", page1.Items[0].Timestamp)
		}

		page2, err := s.FilterTransactions(ctx, model.TransactionFilter{Status: &status}, model.ListOptions{Limit: 1, Offset: 1, IncludeCount: true})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(page2.Items) != 1 {
			t.Fatalf("expected 1 item, got %d", len(page2.Items))
		}
		if page2.Items[0].Timestamp != 1000 {
			t.Errorf("expected timestamp 1000, got %d", page2.Items[0].Timestamp)
		}
	})

	t.Run("no matches returns empty", func(t *testing.T) {
		txType := model.TxTypeTransfer
		result, err := s.FilterTransactions(ctx, model.TransactionFilter{Type: &txType}, model.ListOptions{Limit: 10, IncludeCount: true})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.TotalCount != 0 {
			t.Errorf("expected 0, got %d", result.TotalCount)
		}
		if len(result.Items) != 0 {
			t.Errorf("expected empty items, got %d", len(result.Items))
		}
	})
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/store/ -run TestFilterTransactions -v`
Expected: FAIL — `FilterTransactions` not defined on `*Store`

- [ ] **Step 3: Implement FilterTransactions in the store**

Add the following method to `internal/store/sqlite_transaction.go` after the `ListTransactionsByAccount` method (after line 558):

```go
func (s *Store) FilterTransactions(ctx context.Context, filter model.TransactionFilter, opts model.ListOptions) (*model.ListResult[*model.Transaction], error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	opts = normalizeListOpts(opts)

	needsJoin := filter.AccountID != nil

	var whereClauses []string
	var args []any

	if filter.AccountID != nil {
		whereClauses = append(whereClauses, "s.account_id = ?")
		args = append(args, *filter.AccountID)
	}
	if filter.Type != nil {
		whereClauses = append(whereClauses, "t.type = ?")
		args = append(args, string(*filter.Type))
	}
	if filter.Status != nil {
		whereClauses = append(whereClauses, "t.status = ?")
		args = append(args, int(*filter.Status))
	}
	if filter.StartTime != nil {
		whereClauses = append(whereClauses, "t.timestamp >= ?")
		args = append(args, *filter.StartTime)
	}
	if filter.EndTime != nil {
		whereClauses = append(whereClauses, "t.timestamp <= ?")
		args = append(args, *filter.EndTime)
	}
	if filter.Description != nil {
		whereClauses = append(whereClauses, "t.description LIKE ?")
		args = append(args, "%"+*filter.Description+"%")
	}

	whereSQL := ""
	if len(whereClauses) > 0 {
		whereSQL = "WHERE " + strings.Join(whereClauses, " AND ")
	}

	fromSQL := "FROM transactions t"
	selectDistinct := ""
	countExpr := "COUNT(*)"
	if needsJoin {
		fromSQL = "FROM transactions t INNER JOIN splits s ON t.id = s.transaction_id"
		selectDistinct = "DISTINCT "
		countExpr = "COUNT(DISTINCT t.id)"
	}

	result := &model.ListResult[*model.Transaction]{
		Limit:  opts.Limit,
		Offset: opts.Offset,
	}

	if opts.IncludeCount {
		countQuery := fmt.Sprintf("SELECT %s %s %s", countExpr, fromSQL, whereSQL)
		if err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&result.TotalCount); err != nil {
			return nil, fmt.Errorf("failed to count filtered transactions: %w", err)
		}
	}

	query := fmt.Sprintf(
		"SELECT %st.id, t.timestamp, t.description, t.status, t.external_id, t.type %s %s ORDER BY t.timestamp DESC, t.id DESC LIMIT ? OFFSET ?",
		selectDistinct, fromSQL, whereSQL,
	)
	queryArgs := append(args, opts.Limit, opts.Offset)

	rows, err := s.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to query filtered transactions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items, err := s.scanTransactions(rows)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []*model.Transaction{}
	}
	result.Items = items
	return result, nil
}
```

Make sure `"strings"` is in the import block of `sqlite_transaction.go`. Check the existing imports first — if `strings` is already imported, skip this step.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/store/ -run TestFilterTransactions -v`
Expected: PASS — all subtests green

- [ ] **Step 5: Commit**

```bash
git add internal/store/sqlite_transaction.go internal/store/sqlite_transaction_filter_test.go
git commit -m "feat(store): implement FilterTransactions with dynamic SQL (#114)"
```

---

### Task 4: Add FilterTransactions to Mock and Service

**Files:**
- Modify: `internal/service/testhelper_test.go` (after `ListTransactionsByAccount` mock, ~line 528)
- Modify: `internal/service/transaction_service.go:116-128`
- Create: `internal/service/transaction_filter_test.go`

- [ ] **Step 1: Add FilterTransactions to the mock**

Add the following fields to the `mockTransactionRepo` struct (around line 245-260 in `testhelper_test.go`):

```go
	filterResult *model.ListResult[*model.Transaction]
	filterErr    error
```

Add the mock method after `ListTransactionsByAccount` (after ~line 560):

```go
func (m *mockTransactionRepo) FilterTransactions(_ context.Context, filter model.TransactionFilter, opts model.ListOptions) (*model.ListResult[*model.Transaction], error) {
	if m.filterErr != nil {
		return nil, m.filterErr
	}
	if m.filterResult != nil {
		return m.filterResult, nil
	}
	// Default: return all transactions with basic filtering
	var items []*model.Transaction
	for _, tx := range m.transactions {
		if filter.Type != nil && tx.Type != *filter.Type {
			continue
		}
		if filter.Status != nil && tx.Status != *filter.Status {
			continue
		}
		if filter.StartTime != nil && tx.Timestamp < *filter.StartTime {
			continue
		}
		if filter.EndTime != nil && tx.Timestamp > *filter.EndTime {
			continue
		}
		if filter.AccountID != nil {
			found := false
			for _, s := range m.splits[tx.ID] {
				if s.AccountID == *filter.AccountID {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		if filter.Description != nil {
			if !strings.Contains(strings.ToLower(tx.Description), strings.ToLower(*filter.Description)) {
				continue
			}
		}
		items = append(items, tx)
	}
	if items == nil {
		items = []*model.Transaction{}
	}
	return &model.ListResult[*model.Transaction]{
		Items:      items,
		TotalCount: len(items),
		Limit:      opts.Limit,
		Offset:     opts.Offset,
	}, nil
}
```

Make sure `"strings"` is imported in `testhelper_test.go`.

- [ ] **Step 2: Add the service method**

Add to `internal/service/transaction_service.go` after `ListTransactionHistory` (after line 128):

```go
// FilterTransactions returns a paginated, multi-dimensionally filtered transaction list.
// If filter.AccountID is nil but accountName is provided, it resolves the name first.
func (ts *TransactionService) FilterTransactions(ctx context.Context, filter model.TransactionFilter, opts model.ListOptions) (*model.ListResult[*model.Transaction], error) {
	return ts.txRepo.FilterTransactions(ctx, filter, opts)
}

// FilterTransactionsByAccountName is a convenience wrapper that resolves an
// account name to its ID and sets filter.AccountID before delegating.
func (ts *TransactionService) FilterTransactionsByAccountName(ctx context.Context, accountName string, filter model.TransactionFilter, opts model.ListOptions) (*model.ListResult[*model.Transaction], error) {
	account, err := ts.accRepo.GetAccountByName(ctx, accountName)
	if err != nil {
		return nil, fmt.Errorf("account not found: %w", err)
	}
	filter.AccountID = &account.ID
	return ts.txRepo.FilterTransactions(ctx, filter, opts)
}
```

- [ ] **Step 3: Write the service test**

Create `internal/service/transaction_filter_test.go`:

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

func TestFilterTransactions(t *testing.T) {
	t.Run("delegates to repo with filter", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		svc := newTestTransactionService(accRepo, txRepo)

		txRepo.addTransaction(&model.Transaction{ID: 1, Timestamp: 1000, Description: "Groceries", Status: model.StatusCleared, Type: model.TxTypeExpense}, nil)
		txRepo.addTransaction(&model.Transaction{ID: 2, Timestamp: 2000, Description: "Salary", Status: model.StatusPending, Type: model.TxTypeIncome}, nil)

		txType := model.TxTypeExpense
		result, err := svc.FilterTransactions(context.Background(), model.TransactionFilter{Type: &txType}, model.ListOptions{Limit: 10})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result.Items) != 1 {
			t.Errorf("expected 1 item, got %d", len(result.Items))
		}
		if result.Items[0].Description != "Groceries" {
			t.Errorf("expected Groceries, got %s", result.Items[0].Description)
		}
	})

	t.Run("repo error propagates", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		svc := newTestTransactionService(accRepo, txRepo)

		txRepo.filterErr = errors.New("db failure")

		_, err := svc.FilterTransactions(context.Background(), model.TransactionFilter{}, model.ListOptions{Limit: 10})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestFilterTransactionsByAccountName(t *testing.T) {
	t.Run("resolves account name and filters", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		svc := newTestTransactionService(accRepo, txRepo)

		accRepo.addAccount(&model.Account{ID: 5, Name: "Assets:Bank", Type: model.AccountTypeAsset, Currency: "USD"})

		txRepo.addTransaction(&model.Transaction{ID: 1, Timestamp: 1000, Description: "ATM", Status: model.StatusCleared, Type: model.TxTypeExpense},
			[]*model.Split{{ID: 1, TransactionID: 1, AccountID: 5, Amount: -1000, Currency: "USD"}})
		txRepo.addTransaction(&model.Transaction{ID: 2, Timestamp: 2000, Description: "Salary", Status: model.StatusPending, Type: model.TxTypeIncome},
			[]*model.Split{{ID: 2, TransactionID: 2, AccountID: 99, Amount: 5000, Currency: "USD"}})

		result, err := svc.FilterTransactionsByAccountName(context.Background(), "Assets:Bank", model.TransactionFilter{}, model.ListOptions{Limit: 10})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result.Items) != 1 {
			t.Errorf("expected 1 item, got %d", len(result.Items))
		}
	})

	t.Run("unknown account returns error", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		svc := newTestTransactionService(accRepo, txRepo)

		_, err := svc.FilterTransactionsByAccountName(context.Background(), "Nonexistent", model.TransactionFilter{}, model.ListOptions{Limit: 10})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/service/ -run 'TestFilterTransactions|TestFilterTransactionsByAccountName' -v`
Expected: PASS

Run: `go test ./...`
Expected: PASS (full suite still green)

- [ ] **Step 5: Commit**

```bash
git add internal/service/transaction_service.go internal/service/testhelper_test.go internal/service/transaction_filter_test.go
git commit -m "feat(service): add FilterTransactions and FilterTransactionsByAccountName (#114)"
```

---

### Task 5: Update CLI Transaction List Command

**Files:**
- Modify: `cmd/transaction/list.go`

- [ ] **Step 1: Update the ListProvider interface**

Replace the `ListProvider` interface in `cmd/transaction/list.go` (lines 21-26):

```go
type ListProvider interface {
	FilterTransactions(ctx context.Context, filter model.TransactionFilter, opts model.ListOptions) (*model.ListResult[*model.Transaction], error)
	FilterTransactionsByAccountName(ctx context.Context, accountName string, filter model.TransactionFilter, opts model.ListOptions) (*model.ListResult[*model.Transaction], error)
	GetTransactionDetailsByIDs(ctx context.Context, txs []*model.Transaction, details map[int64]*model.TransactionDetail) (map[int64]*model.TransactionDetail, error)
	BuildTransactionListItems(ctx context.Context, txs []*model.Transaction, details map[int64]*model.TransactionDetail) []model.TransactionListItem
}
```

Note: Check the exact signature of `GetTransactionDetailsByIDs` in the service — it may take `[]*model.Transaction` only. Match whatever is already there.

- [ ] **Step 2: Update listFlags struct**

Replace the `listFlags` struct (lines 28-32):

```go
type listFlags struct {
	Account     string
	Limit       int
	JSON        bool
	Status      string
	Type        string
	From        string
	To          string
	Description string
}
```

- [ ] **Step 3: Add flag registrations**

Add the new flags in `NewListCmd` after the existing flag registrations (after line 63):

```go
	cmd.Flags().StringVar(&flags.Status, "status", "", "Filter by status (pending, cleared, reconciled)")
	cmd.Flags().StringVar(&flags.Type, "type", "", "Filter by type (expense, income, transfer)")
	cmd.Flags().StringVar(&flags.From, "from", "", "Filter from date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&flags.To, "to", "", "Filter to date (YYYY-MM-DD)")
	cmd.Flags().StringVarP(&flags.Description, "description", "d", "", "Filter by description (substring match)")
```

- [ ] **Step 4: Replace fetchTransactions with filter-based logic**

Add a `"time"` import to the file. Replace the `fetchTransactions` method (lines 89-94):

```go
func (r *listRunner) fetchTransactions(ctx context.Context) ([]*model.Transaction, error) {
	filter, err := r.buildFilter()
	if err != nil {
		return nil, err
	}

	opts := model.ListOptions{Limit: r.flags.Limit, IncludeCount: false}

	var result *model.ListResult[*model.Transaction]
	if r.flags.Account != "" {
		result, err = r.svc.FilterTransactionsByAccountName(ctx, r.flags.Account, filter, opts)
	} else {
		result, err = r.svc.FilterTransactions(ctx, filter, opts)
	}
	if err != nil {
		return nil, err
	}
	return result.Items, nil
}

func (r *listRunner) buildFilter() (model.TransactionFilter, error) {
	var filter model.TransactionFilter

	if r.flags.Status != "" {
		s := model.ParseTransactionStatus(r.flags.Status)
		filter.Status = &s
	}
	if r.flags.Type != "" {
		txType, err := model.ParseTransactionType(r.flags.Type)
		if err != nil {
			return filter, err
		}
		filter.Type = &txType
	}
	if r.flags.From != "" {
		t, err := time.Parse(model.DateFormat, r.flags.From)
		if err != nil {
			return filter, fmt.Errorf("invalid --from date: %w", err)
		}
		ts := t.Unix()
		filter.StartTime = &ts
	}
	if r.flags.To != "" {
		t, err := time.Parse(model.DateFormat, r.flags.To)
		if err != nil {
			return filter, fmt.Errorf("invalid --to date: %w", err)
		}
		// End of day
		ts := t.Add(24*time.Hour - time.Second).Unix()
		filter.EndTime = &ts
	}
	if r.flags.Description != "" {
		filter.Description = &r.flags.Description
	}

	return filter, nil
}
```

- [ ] **Step 5: Verify it compiles and all tests pass**

Run: `go build ./cmd/...`
Expected: success

Run: `go test ./...`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add cmd/transaction/list.go
git commit -m "feat(cmd): add --status, --type, --from, --to, --description flags to transaction list (#114)"
```

---

### Task 6: Verify Full Suite and Clean Up

- [ ] **Step 1: Run the full test suite**

Run: `go test ./... -count=1`
Expected: PASS — all packages green

- [ ] **Step 2: Run the build**

Run: `make build`
Expected: produces `./kea_test` binary successfully

- [ ] **Step 3: Smoke test the CLI**

Run: `./kea_test transaction list --help`
Expected: shows `--status`, `--type`, `--from`, `--to`, `--description` flags in help output

- [ ] **Step 4: Run go vet and check for issues**

Run: `go vet ./...`
Expected: no warnings
