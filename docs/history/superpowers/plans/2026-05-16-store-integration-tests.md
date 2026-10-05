# Store Integration Tests Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add integration tests for `internal/store/` that verify every SQL query against a real in-memory SQLite database (closes #60).

**Architecture:** Each test file mirrors its implementation counterpart (e.g., `sqlite_account_test.go` tests `sqlite_account.go`). All tests use the existing `setupTestDB` helper in `sqlite_account_test.go` which creates a temp-dir-backed SQLite with migrations applied. Tests are in `package store_test` (black-box). Each test creates its own DB via `setupTestDB(t)` for isolation.

**Tech Stack:** Go standard `testing`, `testify/assert` + `testify/require`, in-memory SQLite via temp directory.

**Existing coverage (do NOT duplicate):**
- `sqlite_account_test.go`: `RenameAccount` (LIKE wildcards, deep nesting, sibling isolation)
- `sqlite_transaction_pagination_test.go`: `ListTransactions`, `ListTransactionsByAccount` (pagination, count, filter)
- `sqlite_store_test.go`: `DB()` accessor
- `sqlite_pragma_test.go`: WAL mode, busy timeout
- `errors_test.go`: error sentinels

---

### Task 1: Account CRUD Tests

**Files:**
- Modify: `internal/store/sqlite_account_test.go`

Tests to add to the existing file, after the current RenameAccount tests.

- [ ] **Step 1: Write CreateAccount + GetAccountByName test**

```go
func TestCreateAccount_And_GetByName(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	id, err := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "checking", nil)
	require.NoError(t, err)
	assert.Positive(t, id)

	acc, err := s.GetAccountByName(ctx, "Assets:Bank")
	require.NoError(t, err)
	assert.Equal(t, id, acc.ID)
	assert.Equal(t, "Assets:Bank", acc.Name)
	assert.Equal(t, model.AccountTypeAsset, acc.Type)
	assert.Equal(t, "USD", acc.Currency)
	assert.Equal(t, "checking", acc.Description)
	assert.Nil(t, acc.ParentID)
	assert.False(t, acc.IsHidden)
}
```

- [ ] **Step 2: Write CreateAccount duplicate name test**

```go
func TestCreateAccount_DuplicateNameReturnsError(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	_, err := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)

	_, err = s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, repository.ErrAlreadyExists)
}
```

Note: This requires adding `"github.com/hance08/kea/internal/repository"` to the imports.

- [ ] **Step 3: Write GetAccountByID test**

```go
func TestGetAccountByID(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	id, err := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "groceries", nil)
	require.NoError(t, err)

	acc, err := s.GetAccountByID(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, "Expenses:Food", acc.Name)
	assert.Equal(t, model.AccountTypeExpense, acc.Type)
}

func TestGetAccountByID_NotFound(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	_, err := s.GetAccountByID(ctx, 99999)
	require.Error(t, err)
	assert.ErrorIs(t, err, repository.ErrNotFound)
}
```

- [ ] **Step 4: Write GetAccountByName not-found test**

```go
func TestGetAccountByName_NotFound(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	_, err := s.GetAccountByName(ctx, "NoSuchAccount")
	require.Error(t, err)
	assert.ErrorIs(t, err, repository.ErrNotFound)
}
```

- [ ] **Step 5: Write AccountExists test**

```go
func TestAccountExists(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	exists, err := s.AccountExists(ctx, "Assets:Bank")
	require.NoError(t, err)
	assert.False(t, exists)

	_, err = s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)

	exists, err = s.AccountExists(ctx, "Assets:Bank")
	require.NoError(t, err)
	assert.True(t, exists)
}
```

- [ ] **Step 6: Write GetAllAccounts and GetAccountsByType tests**

```go
func TestGetAllAccounts(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	_, err := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)
	_, err = s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)

	accounts, err := s.GetAllAccounts(ctx)
	require.NoError(t, err)
	assert.Len(t, accounts, 2)
	// Ordered by name
	assert.Equal(t, "Assets:Bank", accounts[0].Name)
	assert.Equal(t, "Expenses:Food", accounts[1].Name)
}

func TestGetAccountsByType(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	_, err := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)
	_, err = s.CreateAccount(ctx, "Assets:Cash", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)
	_, err = s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)

	assets, err := s.GetAccountsByType(ctx, model.AccountTypeAsset)
	require.NoError(t, err)
	assert.Len(t, assets, 2)

	expenses, err := s.GetAccountsByType(ctx, model.AccountTypeExpense)
	require.NoError(t, err)
	assert.Len(t, expenses, 1)
}
```

- [ ] **Step 7: Write ParentID, HasChildAccounts, DeleteAccount, UpdateAccountMetadata tests**

```go
func TestCreateAccount_WithParentID(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	parentID, err := s.CreateAccount(ctx, "Assets", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)

	childID, err := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", &parentID)
	require.NoError(t, err)

	child, err := s.GetAccountByID(ctx, childID)
	require.NoError(t, err)
	require.NotNil(t, child.ParentID)
	assert.Equal(t, parentID, *child.ParentID)
}

func TestHasChildAccounts(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	parentID, err := s.CreateAccount(ctx, "Assets", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)

	has, err := s.HasChildAccounts(ctx, parentID)
	require.NoError(t, err)
	assert.False(t, has)

	_, err = s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", &parentID)
	require.NoError(t, err)

	has, err = s.HasChildAccounts(ctx, parentID)
	require.NoError(t, err)
	assert.True(t, has)
}

func TestDeleteAccount(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	id, err := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)

	err = s.DeleteAccount(ctx, id)
	require.NoError(t, err)

	exists, err := s.AccountExists(ctx, "Assets:Bank")
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestDeleteAccount_NotFound(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	err := s.DeleteAccount(ctx, 99999)
	require.Error(t, err)
}

func TestUpdateAccountMetadata(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	id, err := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)

	err = s.UpdateAccountMetadata(ctx, id, "updated desc", true)
	require.NoError(t, err)

	acc, err := s.GetAccountByID(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, "updated desc", acc.Description)
	assert.True(t, acc.IsHidden)
}

func TestUpdateAccountMetadata_NotFound(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	err := s.UpdateAccountMetadata(ctx, 99999, "desc", false)
	require.Error(t, err)
}
```

- [ ] **Step 8: Write AccountHasTransactions test**

```go
func TestAccountHasTransactions(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	assetID, err := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)
	expenseID, err := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)

	has, err := s.AccountHasTransactions(ctx, assetID)
	require.NoError(t, err)
	assert.False(t, has)

	_, err = s.CreateTransactionWithSplits(ctx, model.Transaction{
		Timestamp: 1000, Description: "test", Status: model.StatusPending, Type: model.TxTypeExpense,
	}, []model.Split{
		{AccountID: assetID, Amount: -500, Currency: "USD"},
		{AccountID: expenseID, Amount: 500, Currency: "USD"},
	})
	require.NoError(t, err)

	has, err = s.AccountHasTransactions(ctx, assetID)
	require.NoError(t, err)
	assert.True(t, has)
}
```

- [ ] **Step 9: Write GetAccountBalance and GetAllAccountBalances tests**

```go
func TestGetAccountBalance(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	assetID, err := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)
	expenseID, err := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)

	// No transactions yet — balance should be 0
	bal, err := s.GetAccountBalance(ctx, assetID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), bal)

	_, err = s.CreateTransactionWithSplits(ctx, model.Transaction{
		Timestamp: 1000, Description: "groceries", Status: model.StatusPending, Type: model.TxTypeExpense,
	}, []model.Split{
		{AccountID: assetID, Amount: -2000, Currency: "USD"},
		{AccountID: expenseID, Amount: 2000, Currency: "USD"},
	})
	require.NoError(t, err)

	bal, err = s.GetAccountBalance(ctx, assetID)
	require.NoError(t, err)
	assert.Equal(t, int64(-2000), bal)

	bal, err = s.GetAccountBalance(ctx, expenseID)
	require.NoError(t, err)
	assert.Equal(t, int64(2000), bal)
}

func TestGetAllAccountBalances(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	assetID, err := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)
	expenseID, err := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)

	_, err = s.CreateTransactionWithSplits(ctx, model.Transaction{
		Timestamp: 1000, Description: "early", Status: model.StatusPending, Type: model.TxTypeExpense,
	}, []model.Split{
		{AccountID: assetID, Amount: -500, Currency: "USD"},
		{AccountID: expenseID, Amount: 500, Currency: "USD"},
	})
	require.NoError(t, err)

	_, err = s.CreateTransactionWithSplits(ctx, model.Transaction{
		Timestamp: 2000, Description: "later", Status: model.StatusPending, Type: model.TxTypeExpense,
	}, []model.Split{
		{AccountID: assetID, Amount: -300, Currency: "USD"},
		{AccountID: expenseID, Amount: 300, Currency: "USD"},
	})
	require.NoError(t, err)

	// asOf = 1500 should only include the first transaction
	balances, err := s.GetAllAccountBalances(ctx, 1500)
	require.NoError(t, err)
	assert.Equal(t, int64(-500), balances[assetID])
	assert.Equal(t, int64(500), balances[expenseID])

	// asOf = 2500 should include both
	balances, err = s.GetAllAccountBalances(ctx, 2500)
	require.NoError(t, err)
	assert.Equal(t, int64(-800), balances[assetID])
	assert.Equal(t, int64(800), balances[expenseID])
}
```

- [ ] **Step 10: Run all account tests and verify they pass**

Run: `go test ./internal/store/ -run 'TestCreate|TestGet|TestAccount|TestDelete|TestUpdate|TestHas' -v`
Expected: All PASS

- [ ] **Step 11: Commit**

```bash
git add internal/store/sqlite_account_test.go
git commit -m "test(store): add account CRUD integration tests (closes #60, part 1)"
```

---

### Task 2: Transaction & Split CRUD Tests

**Files:**
- Create: `internal/store/sqlite_transaction_test.go`

New file for transaction/split CRUD tests. Uses the `setupTestDB` helper already defined in `sqlite_account_test.go` (same `store_test` package).

- [ ] **Step 1: Write CreateTransactionWithSplits + GetTransactionByID test**

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

func TestCreateTransactionWithSplits_And_GetByID(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	assetID, err := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)
	expenseID, err := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)

	extID := "ext-123"
	txID, err := s.CreateTransactionWithSplits(ctx, model.Transaction{
		Timestamp:   1000,
		Description: "groceries",
		Status:      model.StatusCleared,
		Type:        model.TxTypeExpense,
		ExternalID:  &extID,
	}, []model.Split{
		{AccountID: assetID, Amount: -2000, Currency: "USD", Memo: "debit"},
		{AccountID: expenseID, Amount: 2000, Currency: "USD", Memo: "credit"},
	})
	require.NoError(t, err)
	assert.Positive(t, txID)

	tx, err := s.GetTransactionByID(ctx, txID)
	require.NoError(t, err)
	assert.Equal(t, txID, tx.ID)
	assert.Equal(t, int64(1000), tx.Timestamp)
	assert.Equal(t, "groceries", tx.Description)
	assert.Equal(t, model.StatusCleared, tx.Status)
	assert.Equal(t, model.TxTypeExpense, tx.Type)
	require.NotNil(t, tx.ExternalID)
	assert.Equal(t, "ext-123", *tx.ExternalID)
}
```

- [ ] **Step 2: Write GetTransactionByID not-found test**

```go
func TestGetTransactionByID_NotFound(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	_, err := s.GetTransactionByID(ctx, 99999)
	require.Error(t, err)
}
```

- [ ] **Step 3: Write GetTransactionsByAccount and GetTransactionsByDateRange tests**

```go
func TestGetTransactionsByAccount(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	assetID, err := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)
	expenseID, err := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)
	otherID, err := s.CreateAccount(ctx, "Expenses:Other", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)

	// 2 txs on assetID+expenseID
	for i := 0; i < 2; i++ {
		_, err := s.CreateTransactionWithSplits(ctx, model.Transaction{
			Timestamp: int64(1000 + i), Description: "food", Status: model.StatusPending, Type: model.TxTypeExpense,
		}, []model.Split{
			{AccountID: assetID, Amount: -100, Currency: "USD"},
			{AccountID: expenseID, Amount: 100, Currency: "USD"},
		})
		require.NoError(t, err)
	}
	// 1 tx on assetID+otherID
	_, err = s.CreateTransactionWithSplits(ctx, model.Transaction{
		Timestamp: 2000, Description: "other", Status: model.StatusPending, Type: model.TxTypeExpense,
	}, []model.Split{
		{AccountID: assetID, Amount: -200, Currency: "USD"},
		{AccountID: otherID, Amount: 200, Currency: "USD"},
	})
	require.NoError(t, err)

	// assetID should see all 3
	txs, err := s.GetTransactionsByAccount(ctx, assetID, 100)
	require.NoError(t, err)
	assert.Len(t, txs, 3)

	// expenseID should see only 2
	txs, err = s.GetTransactionsByAccount(ctx, expenseID, 100)
	require.NoError(t, err)
	assert.Len(t, txs, 2)
}

func TestGetTransactionsByDateRange(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	assetID, err := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)
	expenseID, err := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)

	for _, ts := range []int64{1000, 2000, 3000} {
		_, err := s.CreateTransactionWithSplits(ctx, model.Transaction{
			Timestamp: ts, Description: "tx", Status: model.StatusPending, Type: model.TxTypeExpense,
		}, []model.Split{
			{AccountID: assetID, Amount: -100, Currency: "USD"},
			{AccountID: expenseID, Amount: 100, Currency: "USD"},
		})
		require.NoError(t, err)
	}

	txs, err := s.GetTransactionsByDateRange(ctx, 1500, 2500)
	require.NoError(t, err)
	assert.Len(t, txs, 1)
	assert.Equal(t, int64(2000), txs[0].Timestamp)

	// Inclusive bounds
	txs, err = s.GetTransactionsByDateRange(ctx, 1000, 3000)
	require.NoError(t, err)
	assert.Len(t, txs, 3)
}
```

- [ ] **Step 4: Write GetAllTransactions test**

```go
func TestGetAllTransactions(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	assetID, err := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)
	expenseID, err := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)

	for i := 0; i < 5; i++ {
		_, err := s.CreateTransactionWithSplits(ctx, model.Transaction{
			Timestamp: int64(1000 + i), Description: "tx", Status: model.StatusPending, Type: model.TxTypeExpense,
		}, []model.Split{
			{AccountID: assetID, Amount: -100, Currency: "USD"},
			{AccountID: expenseID, Amount: 100, Currency: "USD"},
		})
		require.NoError(t, err)
	}

	txs, err := s.GetAllTransactions(ctx, 3)
	require.NoError(t, err)
	assert.Len(t, txs, 3)

	txs, err = s.GetAllTransactions(ctx, 0)
	require.NoError(t, err)
	assert.Len(t, txs, 5)
}
```

- [ ] **Step 5: Write UpdateTransactionStatus, UpdateTransactionBasic, DeleteTransaction tests**

```go
func TestUpdateTransactionStatus(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	assetID, err := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)
	expenseID, err := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)

	txID, err := s.CreateTransactionWithSplits(ctx, model.Transaction{
		Timestamp: 1000, Description: "tx", Status: model.StatusPending, Type: model.TxTypeExpense,
	}, []model.Split{
		{AccountID: assetID, Amount: -100, Currency: "USD"},
		{AccountID: expenseID, Amount: 100, Currency: "USD"},
	})
	require.NoError(t, err)

	err = s.UpdateTransactionStatus(ctx, txID, model.StatusCleared)
	require.NoError(t, err)

	tx, err := s.GetTransactionByID(ctx, txID)
	require.NoError(t, err)
	assert.Equal(t, model.StatusCleared, tx.Status)
}

func TestUpdateTransactionStatus_NotFound(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	err := s.UpdateTransactionStatus(ctx, 99999, model.StatusCleared)
	require.Error(t, err)
}

func TestUpdateTransactionBasic(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	assetID, err := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)
	expenseID, err := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)

	txID, err := s.CreateTransactionWithSplits(ctx, model.Transaction{
		Timestamp: 1000, Description: "old", Status: model.StatusPending, Type: model.TxTypeExpense,
	}, []model.Split{
		{AccountID: assetID, Amount: -100, Currency: "USD"},
		{AccountID: expenseID, Amount: 100, Currency: "USD"},
	})
	require.NoError(t, err)

	err = s.UpdateTransactionBasic(ctx, txID, "new desc", 2000, model.StatusCleared, model.TxTypeIncome)
	require.NoError(t, err)

	tx, err := s.GetTransactionByID(ctx, txID)
	require.NoError(t, err)
	assert.Equal(t, "new desc", tx.Description)
	assert.Equal(t, int64(2000), tx.Timestamp)
	assert.Equal(t, model.StatusCleared, tx.Status)
	assert.Equal(t, model.TxTypeIncome, tx.Type)
}

func TestDeleteTransaction(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	assetID, err := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)
	expenseID, err := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)

	txID, err := s.CreateTransactionWithSplits(ctx, model.Transaction{
		Timestamp: 1000, Description: "tx", Status: model.StatusPending, Type: model.TxTypeExpense,
	}, []model.Split{
		{AccountID: assetID, Amount: -100, Currency: "USD"},
		{AccountID: expenseID, Amount: 100, Currency: "USD"},
	})
	require.NoError(t, err)

	err = s.DeleteTransaction(ctx, txID)
	require.NoError(t, err)

	_, err = s.GetTransactionByID(ctx, txID)
	require.Error(t, err)
}

func TestDeleteTransaction_NotFound(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	err := s.DeleteTransaction(ctx, 99999)
	require.Error(t, err)
}
```

- [ ] **Step 6: Write split CRUD tests (CreateSplit, UpdateSplit, DeleteSplit, GetSplitsByTransaction)**

```go
func TestSplitCRUD(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	assetID, err := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)
	expenseID, err := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)

	txID, err := s.CreateTransactionWithSplits(ctx, model.Transaction{
		Timestamp: 1000, Description: "tx", Status: model.StatusPending, Type: model.TxTypeExpense,
	}, []model.Split{
		{AccountID: assetID, Amount: -100, Currency: "USD"},
		{AccountID: expenseID, Amount: 100, Currency: "USD"},
	})
	require.NoError(t, err)

	// GetSplitsByTransaction
	splits, err := s.GetSplitsByTransaction(ctx, txID)
	require.NoError(t, err)
	assert.Len(t, splits, 2)
	assert.Equal(t, int64(-100), splits[0].Amount)
	assert.Equal(t, int64(100), splits[1].Amount)

	// CreateSplit — add a third split
	otherID, err := s.CreateAccount(ctx, "Expenses:Other", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)
	newSplitID, err := s.CreateSplit(ctx, txID, &model.Split{
		AccountID: otherID, Amount: 50, Currency: "USD", Memo: "added",
	})
	require.NoError(t, err)
	assert.Positive(t, newSplitID)

	splits, err = s.GetSplitsByTransaction(ctx, txID)
	require.NoError(t, err)
	assert.Len(t, splits, 3)

	// UpdateSplit
	err = s.UpdateSplit(ctx, newSplitID, otherID, 75, "USD", "updated memo")
	require.NoError(t, err)

	splits, err = s.GetSplitsByTransaction(ctx, txID)
	require.NoError(t, err)
	var updated *model.Split
	for _, sp := range splits {
		if sp.ID == newSplitID {
			updated = sp
		}
	}
	require.NotNil(t, updated)
	assert.Equal(t, int64(75), updated.Amount)
	assert.Equal(t, "updated memo", updated.Memo)

	// DeleteSplit
	err = s.DeleteSplit(ctx, newSplitID)
	require.NoError(t, err)

	splits, err = s.GetSplitsByTransaction(ctx, txID)
	require.NoError(t, err)
	assert.Len(t, splits, 2)
}

func TestUpdateSplit_NotFound(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	err := s.UpdateSplit(ctx, 99999, 1, 100, "USD", "")
	require.Error(t, err)
}

func TestDeleteSplit_NotFound(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	err := s.DeleteSplit(ctx, 99999)
	require.Error(t, err)
}
```

- [ ] **Step 7: Write GetSplitsWithAccounts tests (ByDateRange, ByTransaction, ByTransactionIDs)**

```go
func TestGetSplitsWithAccountsByDateRange(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	assetID, err := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)
	expenseID, err := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)

	tx1ID, err := s.CreateTransactionWithSplits(ctx, model.Transaction{
		Timestamp: 1000, Description: "in range", Status: model.StatusPending, Type: model.TxTypeExpense,
	}, []model.Split{
		{AccountID: assetID, Amount: -100, Currency: "USD"},
		{AccountID: expenseID, Amount: 100, Currency: "USD"},
	})
	require.NoError(t, err)

	_, err = s.CreateTransactionWithSplits(ctx, model.Transaction{
		Timestamp: 3000, Description: "out of range", Status: model.StatusPending, Type: model.TxTypeExpense,
	}, []model.Split{
		{AccountID: assetID, Amount: -200, Currency: "USD"},
		{AccountID: expenseID, Amount: 200, Currency: "USD"},
	})
	require.NoError(t, err)

	result, err := s.GetSplitsWithAccountsByDateRange(ctx, 500, 1500)
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Len(t, result[tx1ID], 2)
	assert.Equal(t, "Assets:Bank", result[tx1ID][0].AccountName)
	assert.Equal(t, model.AccountTypeAsset, result[tx1ID][0].AccountType)
}

func TestGetSplitsWithAccountsByTransaction(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	assetID, err := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)
	expenseID, err := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)

	txID, err := s.CreateTransactionWithSplits(ctx, model.Transaction{
		Timestamp: 1000, Description: "tx", Status: model.StatusPending, Type: model.TxTypeExpense,
	}, []model.Split{
		{AccountID: assetID, Amount: -100, Currency: "USD", Memo: "debit"},
		{AccountID: expenseID, Amount: 100, Currency: "USD", Memo: "credit"},
	})
	require.NoError(t, err)

	details, err := s.GetSplitsWithAccountsByTransaction(ctx, txID)
	require.NoError(t, err)
	assert.Len(t, details, 2)
	assert.Equal(t, "Assets:Bank", details[0].AccountName)
	assert.Equal(t, int64(-100), details[0].Amount)
	assert.Equal(t, "debit", details[0].Memo)
	assert.Equal(t, "Expenses:Food", details[1].AccountName)
}

func TestGetSplitsWithAccountsByTransactionIDs(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	assetID, err := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)
	expenseID, err := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)

	tx1ID, err := s.CreateTransactionWithSplits(ctx, model.Transaction{
		Timestamp: 1000, Description: "tx1", Status: model.StatusPending, Type: model.TxTypeExpense,
	}, []model.Split{
		{AccountID: assetID, Amount: -100, Currency: "USD"},
		{AccountID: expenseID, Amount: 100, Currency: "USD"},
	})
	require.NoError(t, err)

	tx2ID, err := s.CreateTransactionWithSplits(ctx, model.Transaction{
		Timestamp: 2000, Description: "tx2", Status: model.StatusPending, Type: model.TxTypeExpense,
	}, []model.Split{
		{AccountID: assetID, Amount: -200, Currency: "USD"},
		{AccountID: expenseID, Amount: 200, Currency: "USD"},
	})
	require.NoError(t, err)

	result, err := s.GetSplitsWithAccountsByTransactionIDs(ctx, []int64{tx1ID, tx2ID})
	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Len(t, result[tx1ID], 2)
	assert.Len(t, result[tx2ID], 2)

	// Empty input returns empty map
	empty, err := s.GetSplitsWithAccountsByTransactionIDs(ctx, []int64{})
	require.NoError(t, err)
	assert.Empty(t, empty)
}
```

- [ ] **Step 8: Run all transaction tests and verify they pass**

Run: `go test ./internal/store/ -run 'TestCreate.*Split|TestGet.*Transaction|TestUpdate.*Transaction|TestDelete.*Transaction|TestSplit|TestGetSplits' -v`
Expected: All PASS

- [ ] **Step 9: Commit**

```bash
git add internal/store/sqlite_transaction_test.go
git commit -m "test(store): add transaction & split CRUD integration tests (closes #60, part 2)"
```

---

### Task 3: Reconciliation Tests

**Files:**
- Create: `internal/store/sqlite_reconcile_test.go`

- [ ] **Step 1: Write GetUnreconciledTransactionsByAccount test**

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

func TestGetUnreconciledTransactionsByAccount(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	assetID, err := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)
	expenseID, err := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)

	tx1ID, err := s.CreateTransactionWithSplits(ctx, model.Transaction{
		Timestamp: 1000, Description: "groceries", Status: model.StatusPending, Type: model.TxTypeExpense,
	}, []model.Split{
		{AccountID: assetID, Amount: -500, Currency: "USD"},
		{AccountID: expenseID, Amount: 500, Currency: "USD"},
	})
	require.NoError(t, err)

	tx2ID, err := s.CreateTransactionWithSplits(ctx, model.Transaction{
		Timestamp: 2000, Description: "dinner", Status: model.StatusCleared, Type: model.TxTypeExpense,
	}, []model.Split{
		{AccountID: assetID, Amount: -300, Currency: "USD"},
		{AccountID: expenseID, Amount: 300, Currency: "USD"},
	})
	require.NoError(t, err)

	entries, err := s.GetUnreconciledTransactionsByAccount(ctx, assetID)
	require.NoError(t, err)
	assert.Len(t, entries, 2)

	// Ordered by timestamp ASC
	assert.Equal(t, tx1ID, entries[0].ID)
	assert.Equal(t, int64(-500), entries[0].Amount)
	assert.Equal(t, "Expenses:Food", entries[0].OffsetAccount)
	assert.Equal(t, tx2ID, entries[1].ID)
	assert.Equal(t, int64(-300), entries[1].Amount)
}
```

- [ ] **Step 2: Write test verifying reconciled splits are excluded**

```go
func TestGetUnreconciledTransactionsByAccount_ExcludesReconciled(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	assetID, err := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)
	expenseID, err := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)

	tx1ID, err := s.CreateTransactionWithSplits(ctx, model.Transaction{
		Timestamp: 1000, Description: "tx1", Status: model.StatusPending, Type: model.TxTypeExpense,
	}, []model.Split{
		{AccountID: assetID, Amount: -500, Currency: "USD"},
		{AccountID: expenseID, Amount: 500, Currency: "USD"},
	})
	require.NoError(t, err)

	_, err = s.CreateTransactionWithSplits(ctx, model.Transaction{
		Timestamp: 2000, Description: "tx2", Status: model.StatusPending, Type: model.TxTypeExpense,
	}, []model.Split{
		{AccountID: assetID, Amount: -300, Currency: "USD"},
		{AccountID: expenseID, Amount: 300, Currency: "USD"},
	})
	require.NoError(t, err)

	// Reconcile tx1 for assetID
	_, err = s.MarkSplitsReconciledByAccount(ctx, assetID, []int64{tx1ID})
	require.NoError(t, err)

	entries, err := s.GetUnreconciledTransactionsByAccount(ctx, assetID)
	require.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.Equal(t, "tx2", entries[0].Description)
}
```

- [ ] **Step 3: Write multi-account reconciliation isolation test**

```go
func TestGetUnreconciledTransactionsByAccount_MultiAccountIsolation(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	assetID, err := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)
	expenseID, err := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)

	txID, err := s.CreateTransactionWithSplits(ctx, model.Transaction{
		Timestamp: 1000, Description: "shared tx", Status: model.StatusPending, Type: model.TxTypeExpense,
	}, []model.Split{
		{AccountID: assetID, Amount: -500, Currency: "USD"},
		{AccountID: expenseID, Amount: 500, Currency: "USD"},
	})
	require.NoError(t, err)

	// Reconcile only the asset side
	_, err = s.MarkSplitsReconciledByAccount(ctx, assetID, []int64{txID})
	require.NoError(t, err)

	// Asset side should be empty
	assetEntries, err := s.GetUnreconciledTransactionsByAccount(ctx, assetID)
	require.NoError(t, err)
	assert.Empty(t, assetEntries)

	// Expense side should still see it
	expenseEntries, err := s.GetUnreconciledTransactionsByAccount(ctx, expenseID)
	require.NoError(t, err)
	assert.Len(t, expenseEntries, 1)
}
```

- [ ] **Step 4: Write offset_account "(split)" label test**

```go
func TestGetUnreconciledTransactionsByAccount_SplitLabel(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	assetID, err := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)
	expense1ID, err := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)
	expense2ID, err := s.CreateAccount(ctx, "Expenses:Transport", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)

	// Multi-offset transaction (asset → two expense accounts)
	_, err = s.CreateTransactionWithSplits(ctx, model.Transaction{
		Timestamp: 1000, Description: "multi", Status: model.StatusPending, Type: model.TxTypeExpense,
	}, []model.Split{
		{AccountID: assetID, Amount: -800, Currency: "USD"},
		{AccountID: expense1ID, Amount: 500, Currency: "USD"},
		{AccountID: expense2ID, Amount: 300, Currency: "USD"},
	})
	require.NoError(t, err)

	entries, err := s.GetUnreconciledTransactionsByAccount(ctx, assetID)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "(split)", entries[0].OffsetAccount)
}
```

- [ ] **Step 5: Write MarkSplitsReconciledByAccount tests**

```go
func TestMarkSplitsReconciledByAccount(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	assetID, err := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)
	expenseID, err := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)

	tx1ID, err := s.CreateTransactionWithSplits(ctx, model.Transaction{
		Timestamp: 1000, Description: "tx1", Status: model.StatusPending, Type: model.TxTypeExpense,
	}, []model.Split{
		{AccountID: assetID, Amount: -500, Currency: "USD"},
		{AccountID: expenseID, Amount: 500, Currency: "USD"},
	})
	require.NoError(t, err)

	tx2ID, err := s.CreateTransactionWithSplits(ctx, model.Transaction{
		Timestamp: 2000, Description: "tx2", Status: model.StatusPending, Type: model.TxTypeExpense,
	}, []model.Split{
		{AccountID: assetID, Amount: -300, Currency: "USD"},
		{AccountID: expenseID, Amount: 300, Currency: "USD"},
	})
	require.NoError(t, err)

	rowsAffected, err := s.MarkSplitsReconciledByAccount(ctx, assetID, []int64{tx1ID, tx2ID})
	require.NoError(t, err)
	assert.Equal(t, int64(2), rowsAffected)

	// Transaction status should be upgraded to Reconciled
	tx1, err := s.GetTransactionByID(ctx, tx1ID)
	require.NoError(t, err)
	assert.Equal(t, model.StatusReconciled, tx1.Status)

	tx2, err := s.GetTransactionByID(ctx, tx2ID)
	require.NoError(t, err)
	assert.Equal(t, model.StatusReconciled, tx2.Status)
}

func TestMarkSplitsReconciledByAccount_EmptyIDs(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	assetID, err := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)

	rows, err := s.MarkSplitsReconciledByAccount(ctx, assetID, []int64{})
	require.NoError(t, err)
	assert.Equal(t, int64(0), rows)
}
```

- [ ] **Step 6: Write BulkUpdateTransactionStatus tests**

```go
func TestBulkUpdateTransactionStatus(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	assetID, err := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)
	expenseID, err := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)

	tx1ID, err := s.CreateTransactionWithSplits(ctx, model.Transaction{
		Timestamp: 1000, Description: "tx1", Status: model.StatusPending, Type: model.TxTypeExpense,
	}, []model.Split{
		{AccountID: assetID, Amount: -100, Currency: "USD"},
		{AccountID: expenseID, Amount: 100, Currency: "USD"},
	})
	require.NoError(t, err)

	tx2ID, err := s.CreateTransactionWithSplits(ctx, model.Transaction{
		Timestamp: 2000, Description: "tx2", Status: model.StatusPending, Type: model.TxTypeExpense,
	}, []model.Split{
		{AccountID: assetID, Amount: -200, Currency: "USD"},
		{AccountID: expenseID, Amount: 200, Currency: "USD"},
	})
	require.NoError(t, err)

	err = s.BulkUpdateTransactionStatus(ctx, []int64{tx1ID, tx2ID}, model.StatusCleared)
	require.NoError(t, err)

	tx1, err := s.GetTransactionByID(ctx, tx1ID)
	require.NoError(t, err)
	assert.Equal(t, model.StatusCleared, tx1.Status)

	tx2, err := s.GetTransactionByID(ctx, tx2ID)
	require.NoError(t, err)
	assert.Equal(t, model.StatusCleared, tx2.Status)
}

func TestBulkUpdateTransactionStatus_EmptyIDs(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	err := s.BulkUpdateTransactionStatus(ctx, []int64{}, model.StatusCleared)
	require.NoError(t, err)
}

func TestBulkUpdateTransactionStatus_MismatchedRowCount(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	assetID, err := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)
	expenseID, err := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)

	txID, err := s.CreateTransactionWithSplits(ctx, model.Transaction{
		Timestamp: 1000, Description: "tx1", Status: model.StatusPending, Type: model.TxTypeExpense,
	}, []model.Split{
		{AccountID: assetID, Amount: -100, Currency: "USD"},
		{AccountID: expenseID, Amount: 100, Currency: "USD"},
	})
	require.NoError(t, err)

	// Pass one real ID and one non-existent — should error
	err = s.BulkUpdateTransactionStatus(ctx, []int64{txID, 99999}, model.StatusCleared)
	require.Error(t, err)
}
```

- [ ] **Step 7: Write Get/SetLastReconciledBalance tests**

```go
func TestGetSetLastReconciledBalance(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	assetID, err := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)

	// No prior reconciliation — should return 0
	bal, err := s.GetLastReconciledBalance(ctx, assetID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), bal)

	// Set balance
	err = s.SetLastReconciledBalance(ctx, assetID, 5000)
	require.NoError(t, err)

	bal, err = s.GetLastReconciledBalance(ctx, assetID)
	require.NoError(t, err)
	assert.Equal(t, int64(5000), bal)

	// Update (upsert)
	err = s.SetLastReconciledBalance(ctx, assetID, 7500)
	require.NoError(t, err)

	bal, err = s.GetLastReconciledBalance(ctx, assetID)
	require.NoError(t, err)
	assert.Equal(t, int64(7500), bal)
}
```

- [ ] **Step 8: Run all reconciliation tests and verify they pass**

Run: `go test ./internal/store/ -run 'TestGetUnreconciled|TestMarkSplits|TestBulkUpdate|TestGetSetLast' -v`
Expected: All PASS

- [ ] **Step 9: Commit**

```bash
git add internal/store/sqlite_reconcile_test.go
git commit -m "test(store): add reconciliation integration tests (closes #60, part 3)"
```

---

### Task 4: ExecTx (Transaction Manager) Test

**Files:**
- Modify: `internal/store/sqlite_store_test.go`

- [ ] **Step 1: Write ExecTx commit and rollback tests**

Add to `sqlite_store_test.go`, after the existing `TestStore_DB`. Add necessary imports (`"context"`, `"fmt"`, `"github.com/hance08/kea/internal/model"`, `"github.com/hance08/kea/internal/repository"`).

```go
func TestExecTx_CommitsOnSuccess(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	err := s.ExecTx(ctx, func(repo repository.Repository) error {
		_, err := repo.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
		return err
	})
	require.NoError(t, err)

	// Account should be visible outside the transaction
	acc, err := s.GetAccountByName(ctx, "Assets:Bank")
	require.NoError(t, err)
	assert.Equal(t, "Assets:Bank", acc.Name)
}

func TestExecTx_RollsBackOnError(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	err := s.ExecTx(ctx, func(repo repository.Repository) error {
		_, err := repo.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
		if err != nil {
			return err
		}
		return fmt.Errorf("simulated failure")
	})
	require.Error(t, err)

	// Account should NOT exist after rollback
	exists, err := s.AccountExists(ctx, "Assets:Bank")
	require.NoError(t, err)
	assert.False(t, exists)
}
```

Note: `setupTestDB` is defined in `sqlite_account_test.go` and shared across the `store_test` package.

- [ ] **Step 2: Run ExecTx tests and verify they pass**

Run: `go test ./internal/store/ -run 'TestExecTx' -v`
Expected: All PASS

- [ ] **Step 3: Commit**

```bash
git add internal/store/sqlite_store_test.go
git commit -m "test(store): add ExecTx commit/rollback integration tests (closes #60, part 4)"
```

---

### Task 5: Run Full Suite and Final Commit

- [ ] **Step 1: Run the full store test suite**

Run: `go test ./internal/store/ -v -count=1`
Expected: All tests pass (existing + new)

- [ ] **Step 2: Run the full project test suite to check for regressions**

Run: `go test ./... -count=1`
Expected: All tests pass

- [ ] **Step 3: Final commit if any cleanup was needed, then verify clean state**

Run: `git status`
Expected: clean working tree
