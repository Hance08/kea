# Deduplicate Split Validation in UpdateTransactionComplete — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Eliminate duplicated split-balance validation in `UpdateTransactionComplete` by extracting a reusable `ValidateSplitDetailsBalance` method, and fix the TOCTOU window by moving the existing-split-ID validation inside the `ExecTx` callback.

**Architecture:** Add a new `ValidateSplitDetailsBalance` method on `TransactionService` in `transaction_validation.go` that validates both balance and currency for `[]SplitDetail` (analogous to `ValidateSplitsBalance` for `[]Split`). Replace inline validation in both `UpdateTransactionComplete` and `ValidateTransactionEdit` with calls to this new method. Move the existing-split-ID ownership check inside the `ExecTx` callback to eliminate the TOCTOU race.

**Tech Stack:** Go, testify (assert/require)

---

### Task 1: Extract `ValidateSplitDetailsBalance` and add tests

**Files:**
- Modify: `internal/service/transaction_validation.go:38-51` (add new method, keep `ValidateSplitDetailsCurrency`)
- Test: `internal/service/transaction_validation_test.go`

- [ ] **Step 1: Write failing tests for `ValidateSplitDetailsBalance`**

Add to `internal/service/transaction_validation_test.go`, after the `TestValidateSplitsBalance` function:

```go
// ──────────────────────────────────────────────
// ValidateSplitDetailsBalance
// ──────────────────────────────────────────────

func TestValidateSplitDetailsBalance(t *testing.T) {
	svc := newTestTransactionService(newMockAccountRepo(), newMockTransactionRepo())

	makeSplitDetails := func(amounts ...int64) []model.SplitDetail {
		result := make([]model.SplitDetail, len(amounts))
		for i, a := range amounts {
			result[i] = model.SplitDetail{Amount: a, Currency: "USD"}
		}
		return result
	}

	t.Run("balanced splits pass", func(t *testing.T) {
		err := svc.ValidateSplitDetailsBalance(makeSplitDetails(1000, -1000))
		require.NoError(t, err)
	})

	t.Run("three splits balance", func(t *testing.T) {
		err := svc.ValidateSplitDetailsBalance(makeSplitDetails(1000, -600, -400))
		require.NoError(t, err)
	})

	t.Run("imbalanced splits rejected", func(t *testing.T) {
		err := svc.ValidateSplitDetailsBalance(makeSplitDetails(1000, -999))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "splits do not balance")
	})

	t.Run("single split rejected", func(t *testing.T) {
		err := svc.ValidateSplitDetailsBalance(makeSplitDetails(100))
		assert.Error(t, err)
	})

	t.Run("mixed currencies rejected", func(t *testing.T) {
		splits := []model.SplitDetail{
			{Amount: 1000, Currency: "TWD"},
			{Amount: -1000, Currency: "USD"},
		}
		err := svc.ValidateSplitDetailsBalance(splits)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "currency")
	})

	t.Run("same currency passes", func(t *testing.T) {
		splits := []model.SplitDetail{
			{Amount: 1000, Currency: "TWD"},
			{Amount: -1000, Currency: "TWD"},
		}
		err := svc.ValidateSplitDetailsBalance(splits)
		require.NoError(t, err)
	})
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/service/ -run TestValidateSplitDetailsBalance -v`
Expected: FAIL — `ValidateSplitDetailsBalance` does not exist yet.

- [ ] **Step 3: Implement `ValidateSplitDetailsBalance`**

Add this method to `internal/service/transaction_validation.go`, right after the existing `ValidateSplitDetailsCurrency` method (after line 51):

```go
// ValidateSplitDetailsBalance validates that SplitDetail entries sum to zero
// and all use the same currency.
func (ts *TransactionService) ValidateSplitDetailsBalance(splits []model.SplitDetail) error {
	var total int64
	var firstCurrency string

	for _, split := range splits {
		if firstCurrency == "" {
			firstCurrency = split.Currency
		} else if split.Currency != firstCurrency {
			return fmt.Errorf("splits must all use the same currency (got %q and %q)", firstCurrency, split.Currency)
		}
		total += split.Amount
	}

	if total != 0 {
		return fmt.Errorf("splits do not balance: total is %d cents (%.2f), must be 0. "+
			"In double-entry bookkeeping, debits must equal credits",
			total, float64(total)/100.0)
	}

	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/service/ -run TestValidateSplitDetailsBalance -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/service/transaction_validation.go internal/service/transaction_validation_test.go
git commit -m "feat: add ValidateSplitDetailsBalance for []SplitDetail validation (issue #64)"
```

---

### Task 2: Replace inline validation in `UpdateTransactionComplete`

**Files:**
- Modify: `internal/service/transaction_ops.go:274-286`

- [ ] **Step 1: Write a test that verifies the improved error message**

The existing `UpdateTransactionComplete` uses a simpler error message ("splits must balance to zero (current sum: %d)"). After this change, the error message will come from `ValidateSplitDetailsBalance` which uses the richer format. Add this test to `internal/service/transaction_ops_test.go` (or the file containing `UpdateTransactionComplete` tests):

First, find the existing test file:

Run: `grep -rn "TestUpdateTransactionComplete" /Users/hance/programming/kea/internal/service/`

Then add a test case that checks the error message format:

```go
t.Run("unbalanced splits return detailed error", func(t *testing.T) {
	accRepo := newMockAccountRepo()
	accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Bank", Type: model.AccountTypeAsset})
	accRepo.addAccount(&model.Account{ID: 2, Name: "Expenses:Food", Type: model.AccountTypeExpense})
	txRepo := newMockTransactionRepo()
	txRepo.addTransaction(&model.Transaction{ID: 2, Status: model.StatusPending})
	svc := newTestTransactionService(accRepo, txRepo)

	splits := []model.SplitDetail{
		{AccountID: 1, Amount: -1000, Currency: "USD"},
		{AccountID: 2, Amount: 900, Currency: "USD"},
	}
	err := svc.UpdateTransactionComplete(context.Background(), 2, "test", 1000, model.StatusPending, model.TxTypeWithdrawal, splits)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "splits do not balance")
})
```

- [ ] **Step 2: Replace inline balance + currency validation with `ValidateSplitDetailsBalance`**

In `internal/service/transaction_ops.go`, replace lines 274-286:

```go
	// Validate splits balance
	var total int64
	for _, split := range splits {
		total += split.Amount
	}
	if total != 0 {
		return fmt.Errorf("splits must balance to zero (current sum: %d)", total)
	}

	// Validate currency consistency
	if err := ts.ValidateSplitDetailsCurrency(splits); err != nil {
		return err
	}
```

With:

```go
	if err := ts.ValidateSplitDetailsBalance(splits); err != nil {
		return err
	}
```

- [ ] **Step 3: Run all tests to verify nothing breaks**

Run: `go test ./internal/service/ -v`
Expected: ALL PASS

- [ ] **Step 4: Commit**

```bash
git add internal/service/transaction_ops.go internal/service/transaction_ops_test.go
git commit -m "refactor: use ValidateSplitDetailsBalance in UpdateTransactionComplete (issue #64)"
```

---

### Task 3: Replace inline validation in `ValidateTransactionEdit`

**Files:**
- Modify: `internal/service/transaction_validation.go:54-82`

- [ ] **Step 1: Replace inline balance + currency check in `ValidateTransactionEdit`**

In `internal/service/transaction_validation.go`, replace lines 60-70 of `ValidateTransactionEdit`:

```go
	// Check balance
	var total int64
	for _, split := range splits {
		total += split.Amount
	}
	if total != 0 {
		return fmt.Errorf("splits do not balance (sum: %s)", utils.FormatAmount(total))
	}

	if err := ts.ValidateSplitDetailsCurrency(splits); err != nil {
		return err
	}
```

With:

```go
	if err := ts.ValidateSplitDetailsBalance(splits); err != nil {
		return err
	}
```

- [ ] **Step 2: Remove the `utils` import if no longer used**

Check whether `utils` is still referenced in `transaction_validation.go`. If not, remove the import line `"github.com/hance08/kea/internal/utils"`.

- [ ] **Step 3: Run existing `ValidateTransactionEdit` tests**

Run: `go test ./internal/service/ -run TestValidateTransactionEdit -v`
Expected: ALL PASS. The error message format changed ("splits do not balance" is still present in the new format), so existing assertions like `assert.Contains(t, err.Error(), "balance")` should still pass.

- [ ] **Step 4: Run the full test suite**

Run: `go test ./...`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add internal/service/transaction_validation.go
git commit -m "refactor: use ValidateSplitDetailsBalance in ValidateTransactionEdit (issue #64)"
```

---

### Task 4: Remove now-unused `ValidateSplitDetailsCurrency`

**Files:**
- Modify: `internal/service/transaction_validation.go:38-51`

- [ ] **Step 1: Verify `ValidateSplitDetailsCurrency` has no remaining callers**

Run: `grep -rn "ValidateSplitDetailsCurrency" /Users/hance/programming/kea/`

Expected: Only the definition in `transaction_validation.go` should remain. If there are other callers, do NOT delete — skip this task.

- [ ] **Step 2: Remove the method**

Delete the `ValidateSplitDetailsCurrency` method (lines 38-51 of `transaction_validation.go`).

- [ ] **Step 3: Run full test suite**

Run: `go test ./...`
Expected: ALL PASS

- [ ] **Step 4: Commit**

```bash
git add internal/service/transaction_validation.go
git commit -m "refactor: remove unused ValidateSplitDetailsCurrency (issue #64)"
```

---

### Task 5: Move existing-split-ID validation inside `ExecTx` to fix TOCTOU

**Files:**
- Modify: `internal/service/transaction_ops.go:292-333` (move code block inside ExecTx callback)

- [ ] **Step 1: Write a test documenting the TOCTOU concern**

This is hard to test as a race in unit tests, but we can verify the validation still works correctly inside the transaction. Add a test in the `UpdateTransactionComplete` test file:

```go
t.Run("split ID belonging to different transaction rejected", func(t *testing.T) {
	accRepo := newMockAccountRepo()
	accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Bank", Type: model.AccountTypeAsset})
	accRepo.addAccount(&model.Account{ID: 2, Name: "Expenses:Food", Type: model.AccountTypeExpense})
	txRepo := newMockTransactionRepo()
	txRepo.addTransaction(&model.Transaction{ID: 2, Status: model.StatusPending})
	txRepo.addSplit(&model.Split{ID: 10, TransactionID: 2, AccountID: 1, Amount: -500, Currency: "USD"})
	txRepo.addSplit(&model.Split{ID: 11, TransactionID: 2, AccountID: 2, Amount: 500, Currency: "USD"})
	svc := newTestTransactionService(accRepo, txRepo)

	splits := []model.SplitDetail{
		{ID: 10, AccountID: 1, Amount: -500, Currency: "USD"},
		{ID: 99, AccountID: 2, Amount: 500, Currency: "USD"}, // ID 99 does not belong to tx 2
	}
	err := svc.UpdateTransactionComplete(context.Background(), 2, "test", 1000, model.StatusPending, model.TxTypeWithdrawal, splits)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "split ID 99 does not belong to transaction 2")
})
```

- [ ] **Step 2: Move the pre-fetch and validation inside `ExecTx`**

In `internal/service/transaction_ops.go`, remove the block at lines 292-333 (the first `existingSplits` fetch, `existingAccountByID` map construction, duplicate/foreign split ID check, and account selectability check) from outside the `ExecTx` callback.

Move it inside the `ExecTx` callback, right after the `UpdateTransactionBasic` call but before the second `GetSplitsByTransaction` call. Merge the two `GetSplitsByTransaction` calls into one — the one inside `ExecTx` (which reads within the transaction snapshot). The restructured `ExecTx` callback should look like:

```go
	return ts.tm.ExecTx(ctx, func(repo repository.Repository) error {
		if err := repo.UpdateTransactionBasic(ctx, txID, description, timestamp, status, txType); err != nil {
			return err
		}

		existingSplits, err := repo.GetSplitsByTransaction(ctx, txID)
		if err != nil {
			return err
		}

		existingAccountByID := make(map[int64]int64, len(existingSplits))
		existingSplitMap := make(map[int64]*model.Split)
		for _, s := range existingSplits {
			existingAccountByID[s.ID] = s.AccountID
			existingSplitMap[s.ID] = s
		}

		// Reject duplicate or foreign split IDs.
		seenSplitIDs := make(map[int64]bool, len(splits))
		for _, split := range splits {
			if split.ID == 0 {
				continue
			}
			if seenSplitIDs[split.ID] {
				return fmt.Errorf("duplicate split ID %d in input", split.ID)
			}
			seenSplitIDs[split.ID] = true
			if _, ok := existingAccountByID[split.ID]; !ok {
				return fmt.Errorf("split ID %d does not belong to transaction %d", split.ID, txID)
			}
		}

		// Validate accounts; enforce selectability only for new or changed splits.
		for _, split := range splits {
			account, err := ts.accRepo.GetAccountByID(ctx, split.AccountID)
			if err != nil {
				return fmt.Errorf("account ID %d not found", split.AccountID)
			}
			isNew := split.ID == 0
			accountChanged := split.ID != 0 && existingAccountByID[split.ID] != split.AccountID
			if isNew || accountChanged {
				if err := ts.checkAccountSelectable(ctx, account); err != nil {
					return fmt.Errorf("split (account ID %d): %w", split.AccountID, err)
				}
			}
		}

		newSplitMap := make(map[int64]bool)
		for _, split := range splits {
			if split.ID != 0 {
				newSplitMap[split.ID] = true
			}
		}

		// Delete splits that are no longer present
		for id := range existingSplitMap {
			if !newSplitMap[id] {
				if err := repo.DeleteSplit(ctx, id); err != nil {
					return fmt.Errorf("failed to delete split: %w", err)
				}
			}
		}

		// Update existing splits or create new ones
		for _, split := range splits {
			if split.ID == 0 {
				newSplit := &model.Split{
					TransactionID: txID,
					AccountID:     split.AccountID,
					Amount:        split.Amount,
					Currency:      split.Currency,
					Memo:          split.Memo,
				}
				_, err := repo.CreateSplit(ctx, txID, newSplit)
				if err != nil {
					return err
				}
			} else {
				if err := repo.UpdateSplit(ctx, split.ID, split.AccountID, split.Amount, split.Currency, split.Memo); err != nil {
					return err
				}
			}
		}
		return nil
	})
```

- [ ] **Step 3: Run all tests**

Run: `go test ./internal/service/ -v`
Expected: ALL PASS

- [ ] **Step 4: Run full suite including store-level tests**

Run: `go test ./...`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add internal/service/transaction_ops.go internal/service/transaction_ops_test.go
git commit -m "fix: move split-ID validation inside ExecTx to eliminate TOCTOU window (issue #64)"
```
